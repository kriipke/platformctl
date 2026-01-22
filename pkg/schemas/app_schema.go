package schemas

// AppSchemaJSON defines the JSON schema for App manifests.
const AppSchemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "App",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "properties": {
    "apiVersion": {"type": "string", "const": "contextops/v1"},
    "kind": {"type": "string", "const": "App"},
    "metadata": {"type": "object"},
    "spec": {"type": "object"}
  }
}`
