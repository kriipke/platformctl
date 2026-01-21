# ADR-005: Git Browsing via GitHub Contents API + Optional Clone

**Status:** Accepted  
**Date:** 2026-01-21  
**Authors:** Platform Engineering Team  
**Phase:** 1C-3 - Git Integration Service  

---

## Context

ContextOps needs to browse Git repository contents to provide visibility into configuration files, Helm charts, and Kubernetes manifests referenced by ArgoCD applications. This browsing functionality supports troubleshooting, configuration validation, and operational awareness.

### Problem Statement

Git repository browsing presents several technical challenges:

1. **Performance vs. Freshness:** Balance between fast access and up-to-date content
2. **Rate limiting:** Git providers have API rate limits that can impact system performance
3. **Authentication:** Secure access to private repositories across different Git providers
4. **Scalability:** Efficient browsing across many repositories and contexts
5. **Provider diversity:** Support for GitHub, GitLab, and potentially on-premises Git servers

### Requirements

- **Fast file access:** File content retrieval in <2 seconds for common operations
- **Multiple providers:** Primary focus on GitHub with extensibility for others
- **Authentication flexibility:** Support PATs, GitHub Apps, and SSH keys
- **Caching strategy:** Reduce API calls while maintaining reasonable freshness
- **Bulk operations:** Efficient handling when browsing large directory structures
- **Fallback resilience:** Graceful degradation when APIs are unavailable

### Considered Alternatives

#### Alternative 1: Full repository cloning
**Description:** Clone entire repositories locally and browse via filesystem.

**Pros:**
- Complete offline access after initial clone
- No API rate limiting issues
- Full Git history available
- Fast local file system access

**Cons:**
- High storage requirements (full repository copies)
- Slow initial clone for large repositories
- Complex synchronization and cleanup logic
- Security concerns with credential storage

#### Alternative 2: GitHub Contents API only
**Description:** Use only GitHub's REST API for all file access operations.

**Pros:**
- No local storage requirements
- Always current data
- Leverages GitHub's CDN and caching
- Simple implementation

**Cons:**
- Rate limiting constraints (5000 requests/hour)
- Network latency for every operation
- Single point of failure (GitHub API)
- Limited to GitHub repositories only

#### Alternative 3: Git archive streaming
**Description:** Use `git archive` command to stream specific files/directories.

**Pros:**
- Efficient for specific file access
- Works with any Git server
- No full clone required

**Cons:**
- Requires Git binary and command execution
- Complex credential management
- Limited by Git server capabilities
- Challenging error handling

---

## Decision

We will implement a **hybrid approach** that prioritizes GitHub Contents API with strategic fallback to repository cloning using go-git library.

### Architecture Overview

#### Primary Strategy: GitHub Contents API
- Use GitHub's "Get repository content" REST API for single file and directory requests
- Leverage GitHub's built-in caching and CDN distribution
- Implement intelligent caching to minimize API rate limit impact

#### Fallback Strategy: go-git Clone
- Clone repositories using go-git for bulk operations or API failures
- Maintain shallow clones with minimal history
- Implement cleanup policies for storage management

#### Provider Abstraction Layer
```go
type GitClient interface {
    GetFile(repo, path, ref string) (*FileContent, error)
    GetDirectory(repo, path, ref string) (*DirectoryContent, error)
    ListRepositories(org string) ([]*Repository, error)
}

type GitHubClient struct { ... }
type GitLabClient struct { ... }  // Future implementation
```

---

## Rationale

### Why GitHub Contents API First?
- **Performance:** Leverages GitHub's global CDN for fast content delivery
- **Rate limit efficiency:** 5000 requests/hour sufficient for most operations with caching
- **Reliability:** GitHub's infrastructure more reliable than self-hosted clones
- **Simplicity:** REST API easier to implement and debug than Git protocols

### Why go-git Fallback?
- **Rate limit mitigation:** When API limits are exceeded, fall back to cloning
- **Bulk operations:** Efficient for browsing large directory structures
- **Offline capability:** Continue operating when GitHub API is unavailable
- **Provider flexibility:** go-git works with any Git server (GitLab, Bitbucket, self-hosted)

### Why Caching Strategy?
- **Rate limit preservation:** Reduce API calls through intelligent caching
- **Performance improvement:** Cached responses faster than API calls
- **Cost optimization:** Minimize external API usage
- **User experience:** Consistent response times

---

## Consequences

### Positive

1. **Performance**
   - Fast file access via GitHub CDN for common operations
   - Intelligent caching reduces redundant API calls
   - Fallback ensures continued operation under all conditions

2. **Scalability**
   - API-first approach scales well with concurrent users
   - Caching reduces load on both GitHub API and internal systems
   - Clone fallback handles bulk operations efficiently

3. **Reliability**
   - Multiple fallback strategies ensure high availability
   - Graceful degradation when external services are unavailable
   - Error handling preserves partial functionality

4. **Developer Experience**
   - Fast file browsing matches developer expectations
   - Familiar Git workflows and patterns
   - Transparent switching between API and clone strategies

### Negative

1. **Complexity**
   - Two different code paths (API vs clone) to implement and test
   - Cache invalidation logic and TTL management
   - Provider abstraction layer adds indirection

2. **Storage Requirements**
   - Cached content and cloned repositories require disk space
   - Cleanup policies needed to prevent unbounded growth
   - Monitoring required for storage usage

3. **Rate Limit Management**
   - Need sophisticated tracking of GitHub API usage
   - Potential for rate limit exhaustion during high usage
   - Complex logic for switching between strategies

### Technical Debt

1. **Cache Management**
   - Cache invalidation on repository changes
   - TTL tuning for different content types
   - Storage and performance optimization

2. **Authentication Strategy**
   - Token management across different Git providers
   - Credential rotation and renewal
   - Security for stored authentication tokens

3. **Provider Abstraction**
   - GitLab and other provider implementations
   - Feature parity across different providers
   - Provider-specific optimization strategies

---

## Implementation Guidelines

### Phase 1C-3 Implementation

#### GitHub Client with Contents API
```go
type GitHubClient struct {
    client *github.Client
    cache  *cache.Client
    limiter *rate.Limiter
}

func (gc *GitHubClient) GetFile(owner, repo, path, ref string) (*FileContent, error) {
    // Check cache first
    cacheKey := fmt.Sprintf("file:%s:%s:%s:%s", owner, repo, ref, path)
    if cached, found := gc.cache.Get(cacheKey); found {
        return cached.(*FileContent), nil
    }
    
    // Rate limiting check
    if err := gc.limiter.Wait(context.Background()); err != nil {
        return nil, fmt.Errorf("rate limit exceeded: %w", err)
    }
    
    // GitHub API call
    fileContent, _, resp, err := gc.client.Repositories.GetContents(
        context.Background(),
        owner, repo, path,
        &github.RepositoryContentGetOptions{Ref: ref},
    )
    
    if err != nil {
        // Check if rate limited - trigger fallback
        if resp != nil && resp.StatusCode == 403 {
            return gc.fallbackToClone(owner, repo, path, ref)
        }
        return nil, fmt.Errorf("github api error: %w", err)
    }
    
    content := &FileContent{
        Path:    path,
        Content: fileContent.GetContent(),
        SHA:     fileContent.GetSHA(),
        Size:    fileContent.GetSize(),
    }
    
    // Cache result
    gc.cache.Set(cacheKey, content, 5*time.Minute)
    
    return content, nil
}
```

#### go-git Fallback Implementation
```go
func (gc *GitHubClient) fallbackToClone(owner, repo, path, ref string) (*FileContent, error) {
    repoURL := fmt.Sprintf("https://github.com/%s/%s.git", owner, repo)
    
    // Use temporary directory for clone
    tempDir, err := os.MkdirTemp("", fmt.Sprintf("contextops-clone-%s-%s", owner, repo))
    if err != nil {
        return nil, err
    }
    defer os.RemoveAll(tempDir)
    
    // Shallow clone with specific reference
    repository, err := git.PlainClone(tempDir, false, &git.CloneOptions{
        URL:           repoURL,
        Auth:          gc.getAuth(),
        ReferenceName: plumbing.ReferenceName(fmt.Sprintf("refs/heads/%s", ref)),
        SingleBranch:  true,
        Depth:         1,
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to clone repository: %w", err)
    }
    
    // Read file from cloned repository
    worktree, err := repository.Worktree()
    if err != nil {
        return nil, err
    }
    
    filePath := filepath.Join(tempDir, path)
    content, err := os.ReadFile(filePath)
    if err != nil {
        return nil, fmt.Errorf("failed to read file %s: %w", path, err)
    }
    
    return &FileContent{
        Path:    path,
        Content: string(content),
        Size:    len(content),
    }, nil
}
```

#### Intelligent Caching Strategy
```go
type CacheConfig struct {
    // Different TTLs for different content types
    FileTTL       time.Duration // 5 minutes for frequently changing files
    DirectoryTTL  time.Duration // 10 minutes for directory listings
    RepositoryTTL time.Duration // 1 hour for repository metadata
}

func (gc *GitHubClient) getCacheKey(operation, owner, repo, ref, path string) string {
    return fmt.Sprintf("github:%s:%s:%s:%s:%s", operation, owner, repo, ref, path)
}

func (gc *GitHubClient) getCacheTTL(path string) time.Duration {
    // Shorter TTL for frequently changing files
    if strings.Contains(path, "values") || strings.Contains(path, "config") {
        return 2 * time.Minute
    }
    
    // Longer TTL for stable files
    if strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml") {
        return 5 * time.Minute
    }
    
    // Default TTL
    return 3 * time.Minute
}
```

### Rate Limiting Strategy
```go
type RateLimiter struct {
    github    *rate.Limiter  // 5000/hour = ~1.4/second
    reset     time.Time      // When rate limit resets
    remaining int            // Remaining requests
    mutex     sync.RWMutex
}

func (rl *RateLimiter) checkAndUpdate(resp *http.Response) error {
    rl.mutex.Lock()
    defer rl.mutex.Unlock()
    
    if resp != nil {
        if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
            rl.remaining, _ = strconv.Atoi(remaining)
        }
        
        if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
            timestamp, _ := strconv.ParseInt(reset, 10, 64)
            rl.reset = time.Unix(timestamp, 0)
        }
    }
    
    // If less than 10% remaining, start aggressive caching
    if rl.remaining < 500 {
        return fmt.Errorf("rate limit approaching: %d remaining", rl.remaining)
    }
    
    return nil
}
```

---

## Security Considerations

### Authentication Strategy
```go
type GitHubAuth struct {
    Type   string // "token", "app", "ssh"
    Token  string
    AppID  int64
    KeyID  int64
    PrivateKey []byte
}

func (gc *GitHubClient) getAuth() transport.AuthMethod {
    switch gc.auth.Type {
    case "token":
        return &http.BasicAuth{
            Username: "token",
            Password: gc.auth.Token,
        }
    case "ssh":
        publicKey, err := ssh.NewPublicKeysFromFile("git", gc.auth.KeyPath, "")
        if err != nil {
            return nil
        }
        return publicKey
    default:
        return nil
    }
}
```

### Content Security
- **Validate file sizes** to prevent DoS via large file requests
- **Scan content** for sensitive patterns before caching
- **Audit access** to private repositories
- **Encrypt cached content** containing sensitive information

---

## Monitoring and Alerting

### Key Metrics
- **API success rate** by provider and operation type
- **Cache hit ratio** for file and directory requests
- **Rate limit consumption** tracking toward GitHub limits
- **Fallback activation rate** when API limits are reached
- **Clone operation latency** and success rate

### Alert Conditions
- Rate limit consumption > 80% of GitHub allowance
- Cache hit ratio < 70% (indicates caching inefficiency)
- API error rate > 5% over 10 minutes
- Clone fallback activation > 10% of requests
- Storage usage for clones > 80% of allocated space

---

## Evolution Path

### Phase 2 Enhancements
- **GitLab API integration** with similar hybrid approach
- **Webhook integration** for cache invalidation on repository changes
- **Advanced caching** with Redis for distributed caching

### Phase 3 Advanced Features
- **Bulk file operations** with optimized API usage
- **Content search capabilities** across repositories
- **Diff visualization** for configuration changes

---

## References

- [GitHub REST API - Repository Contents](https://docs.github.com/rest/repos/contents)
- [go-git Documentation](https://pkg.go.dev/github.com/go-git/go-git/v5)
- [GitHub Rate Limiting](https://docs.github.com/rest/overview/resources-in-the-rest-api#rate-limiting)
- [GitHub Apps Authentication](https://docs.github.com/developers/apps/building-github-apps/authenticating-with-github-apps)

---

## Related ADRs

- ADR-003: Secrets posture - Defines how Git authentication credentials are managed
- ADR-007: Caching layers and TTL policies - Defines caching strategy for Git content
- ADR-001: Event-driven integration workflows - Git service participates in event-driven architecture