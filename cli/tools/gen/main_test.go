package main

import (
	"go/format"
	"strings"
	"testing"
)

// ── string helpers ────────────────────────────────────────────────────────────

func TestCamelToKebab(t *testing.T) {
	cases := []struct{ in, want string }{
		{"listFunctions", "list-functions"},
		{"getFunction", "get-function"},
		{"updateEnvVars", "update-env-vars"},
		{"createFunction", "create-function"},
		{"id", "id"},
		{"ID", "i-d"},
		{"getVersionDiff", "get-version-diff"},
	}
	for _, c := range cases {
		if got := camelToKebab(c.in); got != c.want {
			t.Errorf("camelToKebab(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSnakeToKebab(t *testing.T) {
	cases := []struct{ in, want string }{
		{"env_vars", "env-vars"},
		{"cron_status", "cron-status"},
		{"name", "name"},
		{"retention_days", "retention-days"},
	}
	for _, c := range cases {
		if got := snakeToKebab(c.in); got != c.want {
			t.Errorf("snakeToKebab(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSnakeToPascal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"env_vars", "EnvVars"},
		{"cron_status", "CronStatus"},
		{"name", "Name"},
		{"retention_days", "RetentionDays"},
		{"id", "Id"},
	}
	for _, c := range cases {
		if got := snakeToPascal(c.in); got != c.want {
			t.Errorf("snakeToPascal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestToPascal(t *testing.T) {
	cases := []struct{ in, want string }{
		{"listFunctions", "ListFunctions"},
		{"createFunction", "CreateFunction"},
		{"", ""},
		{"a", "A"},
	}
	for _, c := range cases {
		if got := toPascal(c.in); got != c.want {
			t.Errorf("toPascal(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEscapeQuotes(t *testing.T) {
	cases := []struct{ in, want string }{
		{`say "hello"`, `say \"hello\"`},
		{"line1\nline2", "line1 line2"},
		{`back\slash`, `back\\slash`},
		{"  trimmed  ", "trimmed"},
	}
	for _, c := range cases {
		if got := escapeQuotes(c.in); got != c.want {
			t.Errorf("escapeQuotes(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// ── deriveCommandName ─────────────────────────────────────────────────────────

func TestDeriveCommandName_Override(t *testing.T) {
	cfg := tagConfigs["Functions"]
	cases := []struct{ opID, want string }{
		{"updateEnvVars", "env"},
		{"updateKV", "kv"},
		{"getNextRun", "next-run"},
	}
	for _, c := range cases {
		if got := deriveCommandName(c.opID, cfg); got != c.want {
			t.Errorf("deriveCommandName(%q) = %q, want %q", c.opID, got, c.want)
		}
	}
}

func TestDeriveCommandName_StripSuffix(t *testing.T) {
	cases := []struct {
		tag  string
		opID string
		want string
	}{
		{"Functions", "listFunctions", "list"},
		{"Functions", "createFunction", "create"},
		{"Functions", "deleteFunction", "delete"},
		{"Versions", "listVersions", "list"},
		{"Versions", "getVersionDiff", "diff"}, // override
		{"Executions", "listExecutions", "list"},
		{"Executions", "getExecution", "get"},
		{"API Tokens", "listTokens", "list"},
		{"API Tokens", "revokeToken", "revoke"},
	}
	for _, c := range cases {
		cfg := tagConfigs[c.tag]
		if got := deriveCommandName(c.opID, cfg); got != c.want {
			t.Errorf("deriveCommandName(%q, %q) = %q, want %q", c.opID, c.tag, got, c.want)
		}
	}
}

// ── schemaGoType / cobraFlagFunc ──────────────────────────────────────────────

func TestSchemaGoType(t *testing.T) {
	cases := []struct {
		schema Schema
		want   string
	}{
		{Schema{Type: "integer"}, "int"},
		{Schema{Type: "boolean"}, "bool"},
		{Schema{Type: "string"}, "string"},
		{Schema{Type: ""}, "string"},
	}
	for _, c := range cases {
		if got := schemaGoType(c.schema); got != c.want {
			t.Errorf("schemaGoType(%q) = %q, want %q", c.schema.Type, got, c.want)
		}
	}
}

func TestCobraFlagFunc(t *testing.T) {
	cases := []struct{ goType, want string }{
		{"int", "IntVar"},
		{"bool", "BoolVar"},
		{"string", "StringVar"},
		{"[]string", "StringVar"}, // fallback
	}
	for _, c := range cases {
		if got := cobraFlagFunc(c.goType); got != c.want {
			t.Errorf("cobraFlagFunc(%q) = %q, want %q", c.goType, got, c.want)
		}
	}
}

// ── mergeParams ───────────────────────────────────────────────────────────────

func TestMergeParams_OpOverridesPath(t *testing.T) {
	pathLevel := []Parameter{
		{Name: "id", In: "path"},
		{Name: "format", In: "query", Description: "path-level"},
	}
	opLevel := []Parameter{
		{Name: "format", In: "query", Description: "op-level"},
	}
	result := mergeParams(pathLevel, opLevel)

	// Should have both id (from path-level) and format (op-level wins)
	if len(result) != 2 {
		t.Fatalf("expected 2 params, got %d: %v", len(result), result)
	}
	// format should be the op-level version
	for _, p := range result {
		if p.Name == "format" && p.Description != "op-level" {
			t.Error("expected op-level format to override path-level")
		}
	}
}

func TestMergeParams_PathLevelAdded(t *testing.T) {
	pathLevel := []Parameter{{Name: "id", In: "path"}}
	opLevel := []Parameter{{Name: "limit", In: "query"}}
	result := mergeParams(pathLevel, opLevel)
	if len(result) != 2 {
		t.Fatalf("expected 2 params, got %d", len(result))
	}
}

func TestMergeParams_NilInputs(t *testing.T) {
	result := mergeParams(nil, nil)
	if len(result) != 0 {
		t.Errorf("expected empty result, got %v", result)
	}
}

// ── resolveRef ────────────────────────────────────────────────────────────────

func TestResolveRef_ResolvesKnownRef(t *testing.T) {
	spec := &Spec{
		Components: Components{
			Schemas: map[string]Schema{
				"MySchema": {Type: "object", Properties: map[string]Schema{
					"name": {Type: "string"},
				}},
			},
		},
	}
	s := Schema{Ref: "#/components/schemas/MySchema"}
	resolved := resolveRef(s, spec)
	if resolved.Type != "object" {
		t.Errorf("expected object type, got %q", resolved.Type)
	}
	if _, ok := resolved.Properties["name"]; !ok {
		t.Error("expected 'name' property in resolved schema")
	}
}

func TestResolveRef_UnknownRef_ReturnsOriginal(t *testing.T) {
	spec := &Spec{Components: Components{Schemas: map[string]Schema{}}}
	s := Schema{Ref: "#/components/schemas/Missing"}
	resolved := resolveRef(s, spec)
	if resolved.Ref != s.Ref {
		t.Errorf("expected original schema back, got %+v", resolved)
	}
}

func TestResolveRef_NoRef_ReturnsOriginal(t *testing.T) {
	spec := &Spec{}
	s := Schema{Type: "string"}
	resolved := resolveRef(s, spec)
	if resolved.Type != "string" {
		t.Errorf("expected string type, got %q", resolved.Type)
	}
}

// ── flattenAllOf ──────────────────────────────────────────────────────────────

func TestFlattenAllOf_MergesProperties(t *testing.T) {
	spec := &Spec{}
	s := Schema{
		AllOf: []Schema{
			{Type: "object", Properties: map[string]Schema{"a": {Type: "string"}}},
			{Type: "object", Properties: map[string]Schema{"b": {Type: "integer"}}},
		},
	}
	flat := flattenAllOf(s, spec)
	if _, ok := flat.Properties["a"]; !ok {
		t.Error("expected 'a' property from first allOf member")
	}
	if _, ok := flat.Properties["b"]; !ok {
		t.Error("expected 'b' property from second allOf member")
	}
}

func TestFlattenAllOf_MergesRequired(t *testing.T) {
	spec := &Spec{}
	s := Schema{
		AllOf: []Schema{
			{Required: []string{"name"}},
			{Required: []string{"code"}},
		},
	}
	flat := flattenAllOf(s, spec)
	required := make(map[string]bool)
	for _, r := range flat.Required {
		required[r] = true
	}
	if !required["name"] {
		t.Error("expected 'name' in required")
	}
	if !required["code"] {
		t.Error("expected 'code' in required")
	}
}

func TestFlattenAllOf_NoAllOf_ReturnsOriginal(t *testing.T) {
	spec := &Spec{}
	s := Schema{Type: "string"}
	flat := flattenAllOf(s, spec)
	if flat.Type != "string" {
		t.Errorf("expected original schema, got %+v", flat)
	}
}

// ── buildFieldInfo ────────────────────────────────────────────────────────────

func TestBuildFieldInfo_StringField(t *testing.T) {
	f := buildFieldInfo("name", "", Schema{Type: "string"}, true, "the name")
	if f.flagName != "name" {
		t.Errorf("flagName = %q", f.flagName)
	}
	if f.goType != "string" {
		t.Errorf("goType = %q", f.goType)
	}
	if f.defaultVal != `""` {
		t.Errorf("defaultVal = %q", f.defaultVal)
	}
	if !f.required {
		t.Error("expected required=true")
	}
	if f.desc != "the name" {
		t.Errorf("desc = %q", f.desc)
	}
}

func TestBuildFieldInfo_IntegerField(t *testing.T) {
	f := buildFieldInfo("limit", "", Schema{Type: "integer", Default: float64(20)}, false, "max items")
	if f.goType != "int" {
		t.Errorf("goType = %q, want int", f.goType)
	}
	if f.defaultVal != "20" {
		t.Errorf("defaultVal = %q, want 20", f.defaultVal)
	}
}

func TestBuildFieldInfo_BooleanField(t *testing.T) {
	f := buildFieldInfo("disabled", "", Schema{Type: "boolean"}, false, "")
	if f.goType != "bool" {
		t.Errorf("goType = %q, want bool", f.goType)
	}
	if f.defaultVal != "false" {
		t.Errorf("defaultVal = %q, want false", f.defaultVal)
	}
}

func TestBuildFieldInfo_MapField(t *testing.T) {
	valSchema := Schema{Type: "string"}
	f := buildFieldInfo("env_vars", "", Schema{
		Type:                 "object",
		AdditionalProperties: &valSchema,
	}, false, "env vars")
	if !f.isMap {
		t.Error("expected isMap=true for object with additionalProperties")
	}
	if f.goType != "[]string" {
		t.Errorf("goType = %q, want []string", f.goType)
	}
	// env_vars gets a shorter flag name
	if f.flagName != "env" {
		t.Errorf("flagName = %q, want env", f.flagName)
	}
}

func TestBuildFieldInfo_MapField_KV(t *testing.T) {
	valSchema := Schema{Type: "string"}
	f := buildFieldInfo("kv", "", Schema{
		Type:                 "object",
		AdditionalProperties: &valSchema,
	}, false, "")
	if !f.isMap {
		t.Error("expected isMap=true")
	}
	// kv does not get a shortened flag name
	if f.flagName != "kv" {
		t.Errorf("flagName = %q, want kv", f.flagName)
	}
}

func TestBuildFieldInfo_CodeField(t *testing.T) {
	f := buildFieldInfo("code", "", Schema{Type: "string"}, true, "Lua code")
	if !f.isCode {
		t.Error("expected isCode=true for 'code' string field")
	}
}

func TestBuildFieldInfo_EnumField_SetsEnumCastType(t *testing.T) {
	f := buildFieldInfo("cron_status", "UpdateFunctionRequest",
		Schema{Type: "string", Enum: []any{"active", "paused"}},
		false, "cron status")
	if f.enumCastType == "" {
		t.Error("expected enumCastType to be set for enum field with schemaName")
	}
	if !strings.Contains(f.enumCastType, "UpdateFunctionRequest") {
		t.Errorf("enumCastType %q should contain schema name", f.enumCastType)
	}
}

func TestBuildFieldInfo_EnumField_NoSchema_NoEnumCast(t *testing.T) {
	// Without a schemaName, no enum cast should be emitted (e.g. query params)
	f := buildFieldInfo("status", "",
		Schema{Type: "string", Enum: []any{"active", "paused"}},
		false, "")
	if f.enumCastType != "" {
		t.Errorf("expected empty enumCastType for query param enum, got %q", f.enumCastType)
	}
}

func TestBuildFieldInfo_SnakeCaseFieldName(t *testing.T) {
	f := buildFieldInfo("cron_status", "", Schema{Type: "string"}, false, "")
	if f.flagName != "cron-status" {
		t.Errorf("flagName = %q, want cron-status", f.flagName)
	}
	if f.goFieldName != "CronStatus" {
		t.Errorf("goFieldName = %q, want CronStatus", f.goFieldName)
	}
}

// ── buildOpInfo ───────────────────────────────────────────────────────────────

func TestBuildOpInfo_SimpleGet(t *testing.T) {
	spec := &Spec{}
	op := &Operation{
		OperationID: "getFunction",
		Summary:     "Get a function",
		Tags:        []string{"Functions"},
		Parameters: []Parameter{
			{Name: "id", In: "path", Schema: Schema{Type: "string"}},
		},
	}
	info, err := buildOpInfo(op, "/api/functions/{id}", "Functions", spec)
	if err != nil {
		t.Fatal(err)
	}
	if info.commandName != "get" {
		t.Errorf("commandName = %q, want get", info.commandName)
	}
	if info.goFuncName != "GetFunction" {
		t.Errorf("goFuncName = %q, want GetFunction", info.goFuncName)
	}
	if len(info.pathArgs) != 1 || info.pathArgs[0].paramName != "id" {
		t.Errorf("pathArgs = %v", info.pathArgs)
	}
	if info.hasBody {
		t.Error("expected no body for GET")
	}
}

func TestBuildOpInfo_IntegerPathArg(t *testing.T) {
	spec := &Spec{}
	op := &Operation{
		OperationID: "getVersion",
		Tags:        []string{"Versions"},
		Parameters: []Parameter{
			{Name: "functionId", In: "path", Schema: Schema{Type: "string"}},
			{Name: "version", In: "path", Schema: Schema{Type: "integer"}},
		},
	}
	info, err := buildOpInfo(op, "/api/functions/{functionId}/versions/{version}", "Versions", spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.pathArgs) != 2 {
		t.Fatalf("expected 2 path args, got %d", len(info.pathArgs))
	}
	versionArg := info.pathArgs[1]
	if versionArg.goType != "int" {
		t.Errorf("version pathArg.goType = %q, want int", versionArg.goType)
	}
}

func TestBuildOpInfo_QueryParams(t *testing.T) {
	spec := &Spec{}
	op := &Operation{
		OperationID: "listFunctions",
		Tags:        []string{"Functions"},
		Parameters: []Parameter{
			{Name: "limit", In: "query", Schema: Schema{Type: "integer", Default: float64(20)}},
			{Name: "offset", In: "query", Schema: Schema{Type: "integer"}},
		},
	}
	info, err := buildOpInfo(op, "/api/functions", "Functions", spec)
	if err != nil {
		t.Fatal(err)
	}
	if len(info.queryFields) != 2 {
		t.Fatalf("expected 2 query fields, got %d", len(info.queryFields))
	}
	if info.queryFields[0].flagName != "limit" {
		t.Errorf("queryFields[0].flagName = %q", info.queryFields[0].flagName)
	}
}

func TestBuildOpInfo_RequestBody(t *testing.T) {
	spec := &Spec{
		Components: Components{
			Schemas: map[string]Schema{
				"CreateFunctionRequest": {
					Type:     "object",
					Required: []string{"name", "code"},
					Properties: map[string]Schema{
						"name": {Type: "string"},
						"code": {Type: "string"},
					},
				},
			},
		},
	}
	op := &Operation{
		OperationID: "createFunction",
		Tags:        []string{"Functions"},
		RequestBody: &RequestBody{
			Content: map[string]MediaType{
				"application/json": {Schema: Schema{Ref: "#/components/schemas/CreateFunctionRequest"}},
			},
		},
	}
	info, err := buildOpInfo(op, "/api/functions", "Functions", spec)
	if err != nil {
		t.Fatal(err)
	}
	if !info.hasBody {
		t.Error("expected hasBody=true")
	}
	if info.bodySchemaName != "CreateFunctionRequest" {
		t.Errorf("bodySchemaName = %q", info.bodySchemaName)
	}
	if len(info.bodyFields) != 2 {
		t.Fatalf("expected 2 body fields, got %d", len(info.bodyFields))
	}
	// Required fields should not be pointers
	for _, f := range info.bodyFields {
		if f.flagName == "name" && f.isPointer {
			t.Error("required field 'name' should not be a pointer")
		}
		if f.flagName == "code" && f.isPointer {
			t.Error("required field 'code' should not be a pointer")
		}
	}
}

// ── generateFile ─────────────────────────────────────────────────────────────

// makeSimpleOp creates a minimal opInfo for code generation tests.
func makeSimpleOp(operationID, commandName, summary string) opInfo {
	return opInfo{
		operationID: operationID,
		commandName: commandName,
		summary:     summary,
		goFuncName:  toPascal(operationID),
	}
}

func TestGenerateFile_ProducesValidGo(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{
		makeSimpleOp("listFunctions", "list", "List all functions"),
	}
	src, err := generateFile("Functions", "Function management", cfg, ops)
	if err != nil {
		t.Fatalf("generateFile error: %v\nsource:\n%s", err, src)
	}
	// Must re-format without error (already formatted, but double-check)
	if _, err := format.Source(src); err != nil {
		t.Errorf("output is not valid Go: %v\nsource:\n%s", err, src)
	}
}

func TestGenerateFile_ContainsPackageDeclaration(t *testing.T) {
	cfg := tagConfigs["Functions"]
	src, _ := generateFile("Functions", "desc", cfg, []opInfo{makeSimpleOp("listFunctions", "list", "")})
	if !strings.Contains(string(src), "package cmd") {
		t.Error("expected 'package cmd' in generated source")
	}
}

func TestGenerateFile_ContainsGeneratedComment(t *testing.T) {
	cfg := tagConfigs["Functions"]
	src, _ := generateFile("Functions", "desc", cfg, []opInfo{makeSimpleOp("listFunctions", "list", "")})
	if !strings.Contains(string(src), "DO NOT EDIT") {
		t.Error("expected DO NOT EDIT comment in generated source")
	}
}

func TestGenerateFile_ContainsParentCommand(t *testing.T) {
	cfg := tagConfigs["Functions"]
	src, _ := generateFile("Functions", "desc", cfg, []opInfo{makeSimpleOp("listFunctions", "list", "")})
	s := string(src)
	if !strings.Contains(s, "functionsCmd") {
		t.Error("expected functionsCmd in generated source")
	}
	if !strings.Contains(s, `rootCmd.AddCommand(functionsCmd)`) {
		t.Error("expected rootCmd.AddCommand(functionsCmd)")
	}
}

func TestGenerateFile_WithPathArg_EmitsExactArgs(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID: "getFunction",
		commandName: "get",
		summary:     "Get a function",
		goFuncName:  "GetFunction",
		pathArgs:    []pathArg{{paramName: "id", goType: "string", displayName: "<id>"}},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "cobra.ExactArgs(1)") {
		t.Error("expected cobra.ExactArgs(1) for one path arg")
	}
}

func TestGenerateFile_WithIntPathArg_EmitsStrconv(t *testing.T) {
	cfg := tagConfigs["Versions"]
	ops := []opInfo{{
		operationID: "getVersion",
		commandName: "get",
		summary:     "Get a version",
		goFuncName:  "GetVersion",
		pathArgs: []pathArg{
			{paramName: "functionId", goType: "string", displayName: "<function-id>"},
			{paramName: "version", goType: "int", displayName: "<version>"},
		},
	}}
	src, err := generateFile("Versions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, `"strconv"`) {
		t.Error("expected strconv import for int path arg")
	}
	if !strings.Contains(s, "strconv.Atoi") {
		t.Error("expected strconv.Atoi call")
	}
}

func TestGenerateFile_WithQueryFields_EmitsParamsStruct(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID: "listFunctions",
		commandName: "list",
		summary:     "List functions",
		goFuncName:  "ListFunctions",
		queryFields: []fieldInfo{
			{flagName: "limit", goVarSuffix: "Limit", goFieldName: "Limit", goType: "int", defaultVal: "20"},
			{flagName: "offset", goVarSuffix: "Offset", goFieldName: "Offset", goType: "int", defaultVal: "0"},
		},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "client.ListFunctionsParams") {
		t.Error("expected Params struct")
	}
	if !strings.Contains(s, `"github.com/dimiro1/lunar/cli/client"`) {
		t.Error("expected client import")
	}
}

func TestGenerateFile_WithBody_EmitsBodyConstruction(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID:  "createFunction",
		commandName:  "create",
		summary:      "Create a function",
		goFuncName:   "CreateFunction",
		hasBody:      true,
		bodyTypeName: "client.CreateFunctionJSONRequestBody",
		bodyFields: []fieldInfo{
			{flagName: "name", goVarSuffix: "Name", goFieldName: "Name", goType: "string", defaultVal: `""`, required: true},
			{flagName: "code", goVarSuffix: "Code", goFieldName: "Code", goType: "string", defaultVal: `""`, required: true, isCode: true},
		},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "client.CreateFunctionJSONRequestBody") {
		t.Error("expected body type in generated source")
	}
	if !strings.Contains(s, `io.ReadAll(os.Stdin)`) {
		t.Error("expected stdin read for code field")
	}
	if !strings.Contains(s, `MarkFlagRequired("name")`) {
		t.Error("expected MarkFlagRequired for name")
	}
}

func TestGenerateFile_WithMapField_EmitsStringArray(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID:  "updateEnvVars",
		commandName:  "env",
		summary:      "Update env vars",
		goFuncName:   "UpdateEnvVars",
		hasBody:      true,
		bodyTypeName: "client.UpdateEnvVarsJSONRequestBody",
		pathArgs:     []pathArg{{paramName: "id", goType: "string", displayName: "<id>"}},
		bodyFields: []fieldInfo{
			{flagName: "env", goVarSuffix: "EnvVars", goFieldName: "EnvVars", goType: "[]string", defaultVal: "nil", isMap: true},
		},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "StringArrayVar") {
		t.Error("expected StringArrayVar for map field")
	}
	if !strings.Contains(s, "strings.SplitN") {
		t.Error("expected strings.SplitN for KEY=VALUE parsing")
	}
}

func TestGenerateFile_WithEnumField_EmitsCast(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID:    "updateFunction",
		commandName:    "update",
		summary:        "Update a function",
		goFuncName:     "UpdateFunction",
		hasBody:        true,
		bodyTypeName:   "client.UpdateFunctionJSONRequestBody",
		bodySchemaName: "UpdateFunctionRequest",
		pathArgs:       []pathArg{{paramName: "id", goType: "string", displayName: "<id>"}},
		bodyFields: []fieldInfo{
			{
				flagName:     "cron-status",
				goVarSuffix:  "CronStatus",
				goFieldName:  "CronStatus",
				goType:       "string",
				defaultVal:   `""`,
				isPointer:    true,
				enumCastType: "client.UpdateFunctionRequestCronStatus",
			},
		},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, "client.UpdateFunctionRequestCronStatus") {
		t.Error("expected enum cast type in generated source")
	}
}

func TestGenerateFile_OptionalField_UsesChangedCheck(t *testing.T) {
	cfg := tagConfigs["Functions"]
	ops := []opInfo{{
		operationID:  "updateFunction",
		commandName:  "update",
		goFuncName:   "UpdateFunction",
		hasBody:      true,
		bodyTypeName: "client.UpdateFunctionJSONRequestBody",
		pathArgs:     []pathArg{{paramName: "id", goType: "string", displayName: "<id>"}},
		bodyFields: []fieldInfo{
			{flagName: "name", goVarSuffix: "Name", goFieldName: "Name", goType: "string", defaultVal: `""`, isPointer: true},
		},
	}}
	src, err := generateFile("Functions", "desc", cfg, ops)
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	if !strings.Contains(s, `cmd.Flags().Changed("name")`) {
		t.Error("expected Changed() check for optional field")
	}
}

func TestGenerateFile_NoClientImport_WhenNoQueryOrBody(t *testing.T) {
	cfg := tagConfigs["API Tokens"]
	ops := []opInfo{
		makeSimpleOp("listTokens", "list", "List tokens"),
	}
	src, _ := generateFile("API Tokens", "desc", cfg, ops)
	// No query fields, no body → client import should not be present
	if strings.Contains(string(src), `"github.com/dimiro1/lunar/cli/client"`) {
		t.Error("client import should be omitted when no query params or body")
	}
}
