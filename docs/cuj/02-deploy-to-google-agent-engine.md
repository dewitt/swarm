# CUJ: Deploying an Agent to Google Agent Engine

## User Persona

Priya has built a successful customer support agent using LangGraph and
Python. She wants to deploy it to production using Google Agent Engine so her
web frontend can query it. She wants the deployment to be repeatable and
version-controlled.

## Journey

### 1. Triggering Deployment

Priya navigates to her agent's project directory and opens the CLI.

```bash
agents
```

> **Priya:** I'm ready to deploy this agent to Google Agent Engine.

### 2. Capability Discovery

The CLI's internal Swarm Agent recognizes the deployment request. It checks
its local context and realizes it doesn't natively know the Google Agent
Engine deployment specifications. It seamlessly downloads the necessary Skill
from the community registry.

> **Swarm CLI:** Loading "Google Agent Engine" deployment skill... I see you
> have a LangGraph agent in Python. I will scaffold a deployment workflow
> using GitHub Actions that builds your agent into a container and deploys it
> to Google Agent Engine.

### 3. GitOps Configuration

The GitOps Agent takes over, generating the required CI/CD files.

> **Swarm CLI:** I have generated a
> `.github/workflows/deploy-agent-engine.yml` file. This workflow requires
> your Google Cloud project ID and service account credentials to be set as
> GitHub Secrets (`GCP_PROJECT_ID` and `GCP_SA_KEY`).
>
> Would you like to review the generated workflow file before I commit it?
> (Y/n)

### 4. Review & Commit

Priya reviews the file and is satisfied.

> **Priya:** Looks good, go ahead and commit it. **Swarm CLI:** Committing
> `.github/workflows/deploy-agent-engine.yml`... Pushing changes to `main`...

### 5. Monitoring

Because `swarm` is integrated with the local repository, it can track the
GitHub Action it just triggered.

> **Swarm CLI:** The deployment workflow has started on GitHub. _\[Spinner:
> Waiting for deployment...\]_ ... Build successful. ... Deployment
> successful.
>
> Your agent is now live! The API endpoint is available at:
> `https://support-agent-xyz123.a.run.app`
