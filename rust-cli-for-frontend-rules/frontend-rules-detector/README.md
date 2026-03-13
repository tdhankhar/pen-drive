# Frontend Rules Detector

A fast Rust CLI tool for detecting React/TypeScript anti-patterns and code quality issues.

## Installation

Binary location: `/home/abhishek/Downloads/experiments/2026-03/pen-drive/rust-cli-for-frontend-rules/frontend-rules-detector/target/release/frontend-rules-detector`

### Quick alias
```bash
alias frd='/home/abhishek/Downloads/experiments/2026-03/pen-drive/rust-cli-for-frontend-rules/frontend-rules-detector/target/release/frontend-rules-detector'
```

## Usage

```bash
frd <PATH> [OPTIONS]
```

### Examples

```bash
# Check all rules (default)
frd /path/to/project

# Check specific rule
frd /path/to/project --rules rendering-conditional-render

# Check multiple rules
frd /path/to/project --rules rendering-conditional-render,js-flatmap-filter
```

## Rules

### 1. `rendering-conditional-render`

Detects unsafe `&&` patterns in JSX that can render falsy values as DOM nodes.

**Unsafe:**
```jsx
{items.length && <ItemList items={items} />}  // 0 renders as DOM
{email && <Email email={email} />}            // "" renders as DOM
{data && <Component data={data} />}           // undefined renders as DOM
```

**Safe patterns:**
```jsx
{isReady && <Component />}                    // Boolean identifier
{items.length > 0 && <List />}                // Comparison
{!disabled && <Button />}                     // Negation
{!!value && <Component />}                    // Double negation
{user?.email && <Email />}                    // Optional chaining
```

**cal.com Results:** 125 violations

---

### 2. `js-flatmap-filter`

Detects `.map().filter()` chains that should use `.flatMap()` instead.

**Pattern:**
```typescript
array.map(fn).filter(Boolean)   // Use flatMap instead
array.map(fn).filter(x => x)     // Same
```

**Optimization:**
```typescript
// Before
array.map(fn).filter(Boolean)

// After
array.flatMap(fn)
```

**cal.com Results:** 6 violations

---

## Build

```bash
cd /path/to/frontend-rules-detector
cargo build --release
```

## Development

Add new rules in `src/main.rs`:

1. Create a check function: `fn check_my_rule(violations, path, content)`
2. Add to main: `if args.rules.contains("my-rule") { check_my_rule(...) }`
3. Update default rules in Args struct

## Performance

- Scans 10,000+ files in < 2 seconds
- Uses Rust + oxc-parser for AST analysis
- Parallel directory traversal

## Output Format

```
Found X violations:

  RULE-NAME  [count]
    file.tsx:line:col – description
    file.tsx:line:col – description
```
