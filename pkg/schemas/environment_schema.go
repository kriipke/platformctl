package schemas

// EnvironmentSchemaJSON defines the JSON schema for Environment manifests.
const EnvironmentSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Environment",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {"type": "string", "const": "contextops/v1"},
    "kind": {"type": "string", "const": "Environment"},
    "metadata": {"type": "object"},
    "spec": {"type": "object"}
  }
}`
