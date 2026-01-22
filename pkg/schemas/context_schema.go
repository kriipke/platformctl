package schemas

// ContextSchemaJSON defines the JSON schema for Context manifests.
const ContextSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "Context",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {"type": "string", "const": "contextops/v1"},
    "kind": {"type": "string", "const": "Context"},
    "metadata": {"type": "object"},
    "spec": {"type": "object"}
  }
}`
