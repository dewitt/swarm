# CUJ: Swarm Collaboration on System Design

## User Persona

Sam is a software architect starting a greenfield project: a new microservices
backend for a retail application. Instead of brainstorming alone, Sam wants to
use a swarm of specialized AI agents to debate and draft the initial system
design document.

## Journey

### 1. Initiating the Swarm Request

Sam opens their terminal in an empty repository and launches the CLI.

```bash
agents
```

> **Sam:** I need to design a new microservices architecture for a retail
> backend. Please ask a swarm of agents to collaborate on this design. We need
> to focus on database choices, API gateway patterns, and security.

### 2. Orchestrating the Swarm

The internal Swarm Agent determines that this requires complex,
multi-perspective generation. It uses the `Swarm Skill` to instantiate three
distinct internal agents:

1. **The Architect:** Focuses on scalability and data flow.
1. **The Security Expert:** Focuses on authentication, authorization, and
   network isolation.
1. **The Data Engineer:** Focuses on database schema, caching, and state
   management.

> **Swarm CLI:** Instantiating a design swarm (Architect, Security Expert,
> Data Engineer)... I will now have them collaborate on a markdown document
> called `system-design.md`.

### 3. Transparent Collaboration

The CLI UI multiplexes the output of the agents, giving Sam a transparent view
into the debate without overwhelming them.

> **\[Architect\]:** Proposing an API Gateway pattern routing to three core
> services: Inventory (Go), Users (Node), and Orders (Go). **\[Data
> Engineer\]:** For Inventory, we should use a NoSQL database like DynamoDB
> for high read throughput, but Orders needs ACID compliance, so PostgreSQL is
> better. **\[Security Expert\]:** We must ensure the API Gateway terminates
> TLS and handles JWT validation before passing traffic to the internal VPC.

### 4. Synthesis and Review

After the swarm concludes its debate, the Swarm Agent synthesizes the
agreed-upon points into the requested file.

> **Swarm CLI:** The swarm has reached a consensus. I have written the
> complete architecture proposal to `system-design.md`, which includes:
>
> - Component diagram definitions (Mermaid JS)
> - Database choices per service (PostgreSQL, DynamoDB)
> - Authentication flow (JWT via API Gateway)
>
> Would you like me to have the Architect agent draft the initial OpenAPI
> specifications for these services based on the design?

### 5. Iteration

Sam is impressed and continues the workflow.

> **Sam:** Yes, generate the OpenAPI specs, and have the Security Expert
> review them before saving.
