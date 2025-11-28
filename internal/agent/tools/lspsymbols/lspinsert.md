Insert content before or after a specific symbol definition using LSP-guided positioning.

**Use these for adding new code adjacent to existing symbols with precise placement.**

<tools>
- **lspinsertbefore**: Insert content before symbol definition
- **lspinsertafter**: Insert content after symbol definition
</tools>

<usage>
- Provide exact symbol name path as anchor
- Specify file containing the anchor symbol
- Provide content to insert
</usage>

<common_use_cases>
**1. Add new method before existing method:**
  Tool: lspinsertbefore
  name_path='MyClass/existingMethod'
  path='src/class.go'
  body='func (c *MyClass) newMethod() { }'

**2. Add helper function after main function:**
  Tool: lspinsertafter
  name_path='main'
  path='cmd/app/main.go'
  body='func helper() { }'

**3. Add field to struct:**
  Tool: lspinsertafter  
  name_path='Config/LastField'
  body='NewField string'
</common_use_cases>

<advantages>
- **Precise positioning**: Relative to specific symbols, not line numbers
- **Structure-aware**: Respects code organization
- **Safe**: No risk of breaking existing code
</advantages>

<tips>
- Use lspfind to identify anchor symbol
- Ensure proper formatting and indentation in body
- Consider code style and organization
- Use lspoverview to verify result
</tips>

<limitations>
- Requires exact anchor symbol path
- Cannot insert within symbol body (use lspreplace instead)
- Formatting responsibility lies with you
</limitations>
