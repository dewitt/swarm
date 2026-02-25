# GitOps and Deployment Workflow

## The GitOps Philosophy

The `agents` project rejects the idea of a proprietary, black-box deployment
engine. Instead, it assumes that version control (Git) is the source of truth
for both code and infrastructure. Deployment is achieved by integrating with
established GitHub (or GitLab/Bitbucket) processes.

## Workflow Integration

When a user asks to deploy an agent, the CLI does not immediately attempt to
push binaries over the network. Instead, the internal GitOps Agent takes the
following steps:

1. **Environment Analysis**: Detects the current Git repository state, CI/CD
   setup, and project framework.
1. **CI/CD Scaffolding**: If a deployment pipeline doesn't exist, the agent
   proposes generating one (e.g., a `.github/workflows/deploy-agent.yml`
   file).
1. **Commit & Push**: The CLI commits the necessary changes (after user
   approval) and pushes to the remote repository.
1. **Monitoring**: The CLI can optionally poll the GitHub Actions API to
   report deployment status back to the terminal.

### Example Interaction

```text
> deploy this agent to AWS Lambda

Agent: I see this is a Python LangGraph agent. You don't currently have a GitHub Action configured for AWS Lambda deployment. 
I have generated a standard serverless deployment workflow. 
Would you like me to commit these changes and push to trigger the deployment? (Y/n)
```

## Environment Management

Agents often require complex environments (API keys, memory stores, database
connections). The CLI integrates with GitOps best practices by:

- Managing `.env` files locally.
- Helping users securely upload secrets to their Git provider (e.g., via
  `gh secret set`).
- Never committing sensitive credentials.

## Reverting and Rollbacks

Because every deployment is tied to a Git commit, rollbacks are native and
simple. If an agent goes rogue or fails in production, the user can simply
say:

> "Revert the last deployment."

The CLI translates this into a `git revert` or an environment rollback via the
CI/CD pipeline, maintaining the Git repository as the strict, auditable ledger
of all agent states.
