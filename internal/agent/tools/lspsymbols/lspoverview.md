Get a high-level structural overview of a file's symbols without reading the entire content.

**Use this to understand file structure before diving into details.**

<token_guidance>
- Low token cost: returns only top-level symbols and line ranges (no bodies)
- Use before lspfind/view to avoid reading whole files
- Default max_answer_chars=150000; typical responses are much smaller
</token_guidance>

<usage>
- Provide file path (required)
- Returns hierarchical list of top-level symbols (classes, functions, exports)
- Shows symbol kinds and locations
</usage>

<advantages>
- **Fast**: No need to read entire file
- **Structured**: See code organization at a glance
- **Hierarchical**: Understand relationships between symbols
- **Selective**: Only top-level symbols, no overwhelming detail
</advantages>

<common_use_cases>
**1. Understand file structure:**
  path='src/services/user.go'
  
**2. Find what a file exports:**
  path='api/handlers.ts'
  
**3. See classes and their structure:**
  path='models/user.py'
  
**4. Navigate large files:**
  Use overview to identify areas of interest, then use lspfind for details
</common_use_cases>

<output_format>
Returns array of symbols with:

- **name_path**: Symbol name
- **kind**: Symbol type (Class, Function, Interface, etc.)
- **relative_path**: File path
- **body_location**: Line range (start_line, end_line)
- **children**: Omitted in overview (use lspfind with depth for details)
</output_format>

<when_to_use>

- **First look** at an unfamiliar file
- Finding exported functions/classes
- Understanding file organization
- Before using lspfind to narrow search
- Checking if file contains certain symbol types
</when_to_use>

<when_not_to_use>

- Need full symbol details or body → use lspfind with include_body=true
- Need hierarchical symbol tree → use lspfind with depth parameter
- Searching across multiple files → use lspfind with directory path
</when_not_to_use>

<tips>
- Pair with view tool to see actual implementation after finding symbols
- Use as a navigation aid in large files
- Compare with grep for quick validation
- Consider max_answer_chars if file has many symbols
</tips>

<limitations>
- Only top-level symbols (no nested methods/functions)
- Requires active LSP server
- File must be indexed by LSP
</limitations>
