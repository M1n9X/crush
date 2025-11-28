**PRIMARY TOOL for finding code definitions** (functions, classes, methods, types, interfaces). Use this INSTEAD of grep when searching for symbols or definitions.

<token_guidance>
- Cheapest definition lookup: omit bodies by default; set max_results small (e.g., 20)
- Use substring=true for lightweight fuzzy name matching without bodies
- Raise include_body or depth only when you must read implementations
- Default max_answer_chars=150000 (~37k tokens); typical responses are far smaller
</token_guidance>

<usage>
- Provide symbol name or pattern to search for (regex enabled by default)
- Optional path to scope search to specific file or directory
- Filter by symbol kind (functions, classes, etc.)
- Optionally include symbol body in results
</usage>

<advantages_over_grep>

- **Structure-aware**: Understands code hierarchy and relationships
- **Precise**: Ignores matches in comments, strings, and non-code content
- **Hierarchical search**: Find symbols by path (e.g., 'pkg/Class/method')
- **Kind filtering**: Search only for specific symbol types
- **Body retrieval**: Get full function/class implementation
</advantages_over_grep>

<common_use_cases>
**1. Find function/class definition:**
  name_path_pattern='FunctionName'
  
**2. Find all methods in a class:**
  name_path_pattern='ClassName/.*'
  is_regex=true
  
**3. Find all functions (no classes/methods):**
  name_path_pattern='.*'
  include_kinds=[12]  # 12 = Function
  
**4. Find interface implementations:**
  name_path_pattern='InterfaceName'
  include_kinds=[11]  # 11 = Interface
  
**5. Get function with full body:**
  name_path_pattern='myFunction'
  include_body=true
</common_use_cases>

<symbol_kinds>
Common kinds (use in include_kinds/exclude_kinds):

- 12 = Function
- 5 = Class  
- 6 = Method
- 11 = Interface
- 23 = Struct
- 10 = Enum
- 13 = Variable
Full list: 1-26 (see LSP specification)
</symbol_kinds>

<pattern_matching>
**By default, uses regex matching:**

- 'Handler' finds HandleRequest, EventHandler, etc.
- '^Handle' finds only symbols starting with Handle
- 'Handler$' finds only symbols ending with Handler

**For exact match:**
  substring=true  # Disables regex, uses substring match
</pattern_matching>

<when_to_use_grep_instead>

- Searching for text in comments or documentation
- Looking for string literals or error messages
- Searching across non-code files (markdown, config, etc.)
- When you need to find text patterns, not code symbols
</when_to_use_grep_instead>

<tips>
- Start with simple name pattern, refine with path if too many results
- Use include_kinds to focus on relevant symbol types
- Set max_results to limit output size
- For iterative symbol exploration, consider using lspoverview first
- Combine with lsprefs to understand how symbols are used
</tips>

<limitations>
- Requires active LSP server for the language
- Performance depends on project size (use path parameter to narrow scope)
- Results limited by max_answer_chars (default 150,000)
</limitations>
