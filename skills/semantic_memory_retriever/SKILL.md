---
Name: semantic_memory_retriever
Description: Retrieves facts from the semantic memory based on a natural language query.
Tools:
  - Name: retrieve_fact
    Description: Retrieves facts from the semantic memory.
    Parameters:
      query: The natural language query string to search for.
      limit: The maximum number of facts to retrieve (default: 5).
---

# Semantic Memory Retriever Skill

This skill allows agents to retrieve relevant facts from the project's semantic memory. It is designed to provide contextual information based on natural language queries, helping agents to make informed decisions and avoid repetitive mistakes.

## How to Use

To use this skill, simply invoke it with a natural language `query`. The skill will then call the `retrieve_fact` tool with your query and return the most relevant facts.

### Parameters

- **`query`** (string, required): The natural language query or statement to search for in the semantic memory. Examples: "project vision", "build command", "file structure".
- **`limit`** (integer, optional): The maximum number of facts to retrieve. If not specified, it defaults to `5`.
  - **Special Case**: If the `query` is exactly 'secret code', the `limit` will be explicitly set to `1`, regardless of the provided or default limit.

### Output

The skill will return the retrieved facts as a formatted list. If no facts are found, it will indicate that no relevant information was found.

## Examples

### Example 1: Basic Fact Retrieval

Retrieve general facts about the project vision.

```
User: retrieve_fact about the project vision
```

### Example 2: Fact Retrieval with a Specific Limit

Retrieve a specific number of facts about the build process.

```
User: retrieve_fact about "build process" with limit 2
```

### Example 3: Special Case: 'secret code'

Retrieve only one fact when the query is 'secret code'.

```
User: retrieve_fact about 'secret code'
```
