Rename a symbol across the entire workspace using LSP's intelligent rename refactoring.

**Use this to safely rename symbols with automatic reference updates.**

<usage>
- Provide exact symbol name path
- Specify file containing the symbol definition
- Provide new name for the symbol
</usage>

<advantages>
- **Workspace-wide**: Automatically renames all references
- **Safe**: Language-aware, won't rename unrelated matches
- **Atomic**: All changes applied together
- **Smart**: Handles imports, qualified names, etc.
</advantages>

<common_use_cases>
**1. Rename function:**
  name_path='oldFunctionName'
  path='utils/helpers.go'
  new_name='newFunctionName'
  
**2. Rename class:**
  name_path='OldClassName'
  path='models/user.go'
  new_name='NewClassName'

# Automatically updates all usages

**3. Rename method:**
  name_path='Service/oldMethod'
  path='services/api.go'  
  new_name='newMethod'
</common_use_cases>

<output>
Returns list of affected files and applies all changes atomically.
</output>

<workflow>
**Step 1**: Find symbol to rename
  Tool: lspfind
  name_path_pattern='oldName'
  
**Step 2**: Verify references
  Tool: lsprefs
  name_path='module/oldName'
  
**Step 3**: Rename
  Tool: lsprename
  name_path='module/oldName'
  new_name='newName'
</workflow>

<tips>
- Verify symbol location with lspfind first
- Check references with lsprefs to understand impact
- New name should follow language naming conventions
- Review affected files after rename
- Test/compile after to ensure correctness
</tips>

<limitations>
- Requires exact symbol name path
- Depends on LSP server's rename capability
- May not handle complex macro/template cases
- Cannot rename across different symbol types
</limitations>
