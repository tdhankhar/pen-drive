# Manual Rule Exploration Prompt

## Objective
Manually search and document violations of two frontend rules in a React codebase.

## Context
We're building an automated rule detector CLI. Before finalizing the CLI implementation, we need to manually validate that our rules correctly identify real violations. This helps us verify:
1. The rules actually catch real anti-patterns in production code
2. The severity/frequency of violations
3. Any false positives or edge cases

## Target Repository
`/home/abhishek/Downloads/chrome-456/delits-2026-02/cal.com-6.2.0`

## Reference Documentation
- React patterns and best practices: `/home/abhishek/Downloads/experiments/2026-03/pen-drive/rust-cli-for-frontend-rules/frontend-rules-detector/docs/react-skills/`
- Rule specifications: `/home/abhishek/Downloads/experiments/2026-03/pen-drive/rust-cli-for-frontend-rules/frontend-rules-detector/docs/work/001-plan.md`

## Rules to Search For

### Rule 1: `barrel-imports`
**Description:** Importing named exports from large/heavy libraries instead of using tree-shaking or dynamic imports.

**Affected Libraries:**
- `lucide-react`
- `@mui/material`
- `@mui/icons-material`
- `@tabler/icons-react`
- `react-icons`
- `lodash`
- `date-fns`

**Pattern to Find:**
```javascript
// BAD - barrel import with named exports
import { Icon1, Icon2, Icon3 } from 'lucide-react';
import { Button, Card, Dialog } from '@mui/material';
import { map, filter, reduce } from 'lodash';

// GOOD - dynamic or proper tree-shaking
import dynamic from 'next/dynamic';
const Icon1 = dynamic(() => import('lucide-react').then(m => m.Icon1));
import Button from '@mui/material/Button';
```

**Search Strategy:**
1. Search for import statements from the listed packages
2. Check if they use named imports (not default imports)
3. Document file path, line number, and which symbols are imported
4. Count occurrences

---

### Rule 2: `rendering-conditional-render`
**Description:** Using `&&` operator for conditional rendering in JSX, which can cause falsy values to render as DOM nodes (like `0`, `false`, empty arrays).

**Pattern to Find:**
```javascript
// BAD - falsy values can render
{items.length && <ItemList items={items} />}
{user && <UserProfile user={user} />}
{data.config && <Config config={data.config} />}

// GOOD - explicit boolean checks or ternary
{items.length > 0 && <ItemList items={items} />}
{user ? <UserProfile user={user} /> : null}
{isReady && <Component />}
```

**Search Strategy:**
1. Search for `&&` operators inside JSX curly braces `{ ... }`
2. Check what's on the left side of `&&`
3. Exclude safe patterns:
   - Comparisons: `x > 0`, `x === true`, `x !== null`
   - Boolean identifiers: `isLoading`, `hasError`, `canEdit`
4. Document occurrences of potentially unsafe patterns
5. Count violations

---

## Deliverables

For each rule, provide:
1. **Count:** Total number of violations found
2. **Examples:** Show 3-5 representative examples with:
   - File path
   - Line number(s)
   - The exact code snippet
3. **Severity Assessment:** How common is this issue?
4. **False Positives:** Any safe patterns flagged by mistake?

## Output Format

```markdown
## Rule 1: barrel-imports
**Total Violations Found:** X

### Examples:
1. File: `src/components/Header.tsx:45`
   ```javascript
   import { Menu, Bell, Settings } from 'lucide-react';
   ```

2. File: `src/pages/dashboard.tsx:12`
   ```javascript
   import { map, filter } from 'lodash';
   ```

...

### Notes:
- Pattern frequency: High/Medium/Low
- False positives encountered: [describe]
```

---

## Tips for Thorough Search

- Use file search patterns: `**/*.tsx`, `**/*.ts`, `**/*.jsx`, `**/*.js`
- For barrel-imports: Grep for exact package names
- For conditional-render: Look for `{` followed by identifier/expression, then `&&`
- Pay attention to minified or formatted code
- Don't count comments or strings that contain the patterns
