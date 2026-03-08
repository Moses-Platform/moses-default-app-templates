package model

// Capability represents a Moses platform capability
type Capability struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Icon        string   `json:"icon"`
	Details     []string `json:"details"`
}

var capabilities = []Capability{
	{
		ID:          "mcp-tools",
		Title:       "MCP Tools",
		Description: "Model Context Protocol tool system with 56+ tools across 5 profiles",
		Category:    "core",
		Icon:        "wrench",
		Details: []string{
			"Unified CRUD: moses_query, moses_create, moses_update, moses_delete, moses_batch",
			"Workspace Tools: Dynamic MCP proxy generation from OpenAPI specs",
			"Tool Profiles: external-minimal, moses-manager-full, agent-execution, build-callback, autonomous",
			"Permission-based access: RBAC enforcement at tool level",
			"Real-time context: Tools receive tenant, user, and chart IDs via headers",
		},
	},
	{
		ID:          "deployment-pipeline",
		Title:       "Deployment Pipeline",
		Description: "Dual-pipeline deployment system for apps and workspace tools",
		Category:    "infrastructure",
		Icon:        "rocket",
		Details: []string{
			"Agent Execution Pipeline: moses_agent_submit_completed → Kaniko → Helm → Ingress",
			"Workspace Tools Pipeline: Git clone → Config parse → Build → Deploy → OpenAPI discovery",
			"Kaniko builds: In-cluster Docker builds without Docker daemon",
			"Helm deployments: Multi-service charts with auto-generated values",
			"Health verification: Automatic probes and readiness checks",
			"Ingress routing: Path-based routing with TLS support",
		},
	},
	{
		ID:          "agent-workspace",
		Title:       "Agent Workspace",
		Description: "Isolated development environments with Claude Code runtime",
		Category:    "agents",
		Icon:        "terminal",
		Details: []string{
			"Pod provisioning: Dedicated K8s pods per agent execution",
			"Skill injection: SKILL.md files embedded at build time",
			"Tool access: MCP server with profile-based tool filtering",
			"Git integration: Shallow clone with credential injection",
			"Runtime adapters: Claude Code CLI, Claude SDK, OpenCode",
			"OAuth + API keys: Credential providers for external services",
		},
	},
	{
		ID:          "authentication",
		Title:       "Authentication & Authorization",
		Description: "Multi-layer auth with OIDC, JWT, and API keys",
		Category:    "security",
		Icon:        "shield-check",
		Details: []string{
			"OIDC/JWT: Keycloak integration with forward auth",
			"API Keys: SHA256-hashed keys for MCP clients",
			"OAuth Tokens: Runtime credential injection for agents",
			"Forward Auth: Ingress-level authentication via moses-backend",
			"Session Management: Secure cookie-based sessions",
			"SSO Support: Enterprise identity provider integration",
		},
	},
	{
		ID:          "multi-tenancy",
		Title:       "Multi-Tenancy & RBAC",
		Description: "Complete tenant isolation with 4-tier role hierarchy",
		Category:    "security",
		Icon:        "users",
		Details: []string{
			"Tenant Isolation: Database, Store, API, and RBAC layers",
			"Role Hierarchy: GlobalAdmin > TenantAdmin > Editor > Viewer",
			"Permission System: Granular resource:action permissions",
			"Namespace Isolation: K8s NetworkPolicy + ResourceQuota",
			"Query Filtering: Automatic tenant_id enforcement at store layer",
			"Audit Logging: All operations tracked with tenant context",
		},
	},
	{
		ID:          "openapi-discovery",
		Title:       "OpenAPI Discovery",
		Description: "Automatic API endpoint discovery and MCP tool generation",
		Category:    "integration",
		Icon:        "search",
		Details: []string{
			"Auto-probe: 11 standard OpenAPI paths (swagger.json, openapi.yaml, etc.)",
			"Port fallback: Try service port + alternate ports (3000, 4000, etc.)",
			"Spec parsing: JSON/YAML OpenAPI 3.x parsing",
			"Tool generation: workspace_{toolKey}_{operationId} MCP tools",
			"Dynamic proxy: WorkspaceToolProxy routes calls to service endpoints",
			"Schema validation: Input/output validation from OpenAPI schemas",
		},
	},
	{
		ID:          "workspace-tools",
		Title:       "Workspace Tools",
		Description: "Marketplace of deployable tools that extend workspace capabilities via MCP",
		Category:    "integration",
		Icon:        "puzzle",
		Details: []string{
			"Tool Marketplace: Register, deploy, and manage tools per workspace",
			"Kaniko Builds: In-cluster Docker image builds from git repos",
			"Dynamic MCP Proxy: Auto-generated MCP tools from OpenAPI specs",
			"Per-chart Deployment: Tools deployed to specific project namespaces",
			"GitLab Integration: Link tools to GitLab repos for CI/CD",
			"Permission Scoping: Tools scoped by tenant and chart permissions",
		},
	},
}

// GetAllCapabilities returns all capabilities
func GetAllCapabilities() []Capability {
	return capabilities
}

// GetCapabilityByID returns a specific capability by ID
func GetCapabilityByID(id string) *Capability {
	for i := range capabilities {
		if capabilities[i].ID == id {
			return &capabilities[i]
		}
	}
	return nil
}
