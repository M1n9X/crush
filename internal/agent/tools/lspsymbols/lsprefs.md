Find all references/usages of a specific symbol using LSP's precise reference tracking.

**Use this when you need to see WHERE and HOW a symbol is used** across your codebase.

<usage>
- Provide exact symbol name path (e.g., 'MyClass/myMethod')
- Specify file containing the symbol definition
- Optionally filter references by containing symbol kind
- Control whether to include import statements
</usage>

<advantages_over_grep>

- **Semantically precise**: Only finds actual references, not text matches
- **Context-aware**: Provides containing symbol information for each reference
- **Language-aware**: Understands qualified names (pkg.Class.method vs other.Class.method)
- **Filtered results**: Exclude imports, filter by symbol kind
</advantages_over_grep>

<common_use_cases>
**1. Find where function is called:**
  name_path='handleRequest'
  path='internal/handlers/request.go'
  
**2. Find usages excluding imports:**
  name_path='UserService'
  path='services/user.go'
  include_imports=false  # (default)
  
**3. Find only references in methods:**
  name_path='validate'
  path='validators/main.go'
  include_kinds=[6]  # 6 = Method
  
**4. Find references including the definition:**
  name_path='Config'
  path='config/config.go'
  include_self=true
</common_use_cases>

<output_format>
Returns list of references with:

- **name_path**: Path of containing symbol
- **kind**: Type of containing symbol (Function, Method, etc.)
- **relative_path**: File containing the reference
- **body_location**: Line range of containing symbol
- **content_around_reference**: Code snippet showing usage context
</output_format>

<when_to_use_grep_instead>

- Don't know the exact symbol name
- Need to find text patterns or comments
- Symbol not indexed by LSP
- Searching in non-code files
</when_to_use_grep_instead>

<tips>
- Use lspfind first to locate the symbol definition
- name_path should match the symbol's hierarchical path (e.g., 'Class/method')
- Set max_results to limit large result sets
- Combine exclude_kinds and include_kinds for precise filtering
- Use include_imports=false (default) to focus on actual usages
</tips>

<workflow_example>
**Step 1**: Find the symbol
  Tool: lspfind
  name_path_pattern='UserService'
  
**Step 2**: Get its references  
  Tool: lsprefs
  name_path='providers/UserService'
  path='providers/service.go'
</workflow_example>

<limitations>
- Requires exact symbol name path (use lspfind to discover)
- Requires active LSP server
- Results depend on LSP server's reference resolution capability
- May not find dynamic/reflection-based references
</limitations>
