Replace the entire body of a symbol (function, method, class) while preserving its signature.

**Use this for surgical code modifications when you know exactly which symbol to modify.**

<usage>
- Provide exact symbol name path (e.g., 'MyClass/myMethod')
- Specify file containing the symbol
- Provide new body text (without signature)
</usage>

<advantages>
- **Signature-preserving**: Only replaces body, keeps declaration
- **Precise**: No risk of modifying wrong symbol
- **Structure-aware**: Uses LSP ranges, not text matching
</advantages>

<common_use_cases>
**1. Replace function implementation:**
  name_path='processData'
  path='utils/data.go'
  body='return data.transform()'  # New implementation

**2. Update method body:**
  name_path='UserService/validate'
  path='services/user.go'
  body='...new validation logic...'

**3. Fix bug in specific function:**
  First use lspfind to locate, then replace body
</common_use_cases>

<workflow>
**Step 1**: Find the symbol
  Tool: lspfind
  name_path_pattern='myFunction'
  include_body=true  # See current implementation
  
**Step 2**: Replace the body
  Tool: lspreplace
  name_path='module/myFunction'
  path='src/module.go'
  body='// new implementation'
</workflow>

<tips>
- Verify symbol path with lspfind first
- New body should NOT include signature/declaration
- Use view tool after to verify changes
- Test compilation after replacement
</tips>

<limitations>
- Requires exact symbol name path
- Only replaces body, not signature
- For signature changes, use lsprename or edit manually
</limitations>
