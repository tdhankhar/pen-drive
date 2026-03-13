
  ---
  AST-Codifiable Rules Analysis

  Here's my assessment of what can be detected deterministically via oxc-parser, organized by confidence tier.

  ---
  TIER 1: High Confidence — Direct AST Pattern Match

  These have clear, unambiguous structural signatures at the AST level.

  ┌──────────────────────────────┬─────────────────────────────────────┬─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │             Rule             │                File                 │                                                            AST Check                                                            │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ No forwardRef (React 19)     │ react19-no-forwardref               │ ImportSpecifier named forwardRef from 'react'; OR CallExpression where callee is Identifier('forwardRef')                       │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ No inline component defs     │ rerender-no-inline-components       │ FunctionDeclaration/VariableDeclarator(ArrowFunctionExpression) inside another function body, where the inner name starts with  │
  │                              │                                     │ uppercase AND its return type/body contains JSXElement                                                                          │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Memo with non-primitive      │ rerender-memo-with-default-value    │ memo(fn) wrapping a component where destructured props have defaults that are ObjectExpression, ArrayExpression, or             │
  │ default                      │                                     │ ArrowFunctionExpression (not Literal/Identifier)                                                                                │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Trivial expression in        │ rerender-simple-expression-in-memo  │ useMemo(() => expr, deps) where expr is a single BinaryExpression or LogicalExpression with ≤2 operators and no function calls  │
  │ useMemo                      │                                     │                                                                                                                                 │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Lazy state init              │ rerender-lazy-state-init            │ useState(callExpr) where the argument is a CallExpression (not ArrowFunctionExpression/FunctionExpression/Literal/Identifier)   │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Barrel file imports          │ bundle-barrel-imports               │ ImportDeclaration with named specifiers (not default) from known packages: lucide-react, @mui/material, @mui/icons-material,    │
  │                              │                                     │ @tabler/icons-react, react-icons, lodash, date-fns                                                                              │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Render-prop props            │ patterns-children-over-render-props │ FunctionDeclaration/ArrowFunctionExpression with a props param where destructured property names match /^render[A-Z]/ and their │
  │                              │                                     │  type is a function type                                                                                                        │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Boolean prop proliferation   │ architecture-avoid-boolean-props    │ Function component where ≥3 destructured prop names match `/^(is                                                                │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ map().filter(Boolean)        │ js-flatmap-filter                   │ CallExpression(.filter) where callee is MemberExpression on another CallExpression(.map), and filter arg is                     │
  │                              │                                     │ Identifier('Boolean') or ArrowFunctionExpression(x => x)                                                                        │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Mutating .sort()             │ js-tosorted-immutable               │ CallExpression(.sort) inside useMemo or component render where the receiver is NOT a SpreadElement copy ([...x]) or .slice()    │
  │                              │                                     │ first                                                                                                                           │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ RegExp inside component      │ js-hoist-regexp                     │ new RegExp(...) CallExpression inside a component function body, NOT wrapped in a useMemo call, NOT at module scope             │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Passive event listeners      │ client-passive-event-listeners      │ addEventListener(event, handler) calls where first arg is 'scroll'/'touchmove'/'touchstart'/'wheel' AND third arg is missing OR │
  │                              │                                     │  is ObjectExpression without passive: true                                                                                      │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Script without defer/async   │ rendering-script-defer-async        │ JSXOpeningElement with name 'script' missing defer and async attributes                                                         │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ && falsy render              │ rendering-conditional-render        │ JSXExpressionContainer containing LogicalExpression with operator && where the left side is not a comparison expression (>, <,  │
  │                              │                                     │ ===, !==, >=, <=) or boolean-typed identifier (heuristic: not prefixed with is/has)                                             │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ LocalStorage in component    │ js-cache-storage                    │ CallExpression localStorage.getItem OR sessionStorage.getItem inside a component function body (not inside a                    │
  │ body                         │                                     │ useCallback/useMemo/event handler)                                                                                              │
  ├──────────────────────────────┼─────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Non-functional setState in   │ rerender-functional-setstate        │ Inside useCallback, setX(expr) where expr references the corresponding state variable directly (e.g., setItems([...items, x])   │
  │ useCallback                  │                                     │ where items comes from useState)                                                                                                │
  └──────────────────────────────┴─────────────────────────────────────┴─────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  TIER 2: Medium Confidence — Multi-Pattern / Heuristic

  Detectable but may require tracking variable references across multiple statements.

  ┌─────────────────────────────────┬───────────────────────────────────┬────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │              Rule               │               File                │                                                           AST Check                                                            │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Event in useEffect → move to    │ rerender-move-effect-to-event     │ useEffect(() => { if (stateFlag) { sideEffect() } }, [stateFlag]) — useEffect body is an if-check on a useState variable then  │
  │ handler                         │                                   │ a non-state call                                                                                                               │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Narrow effect dependencies      │ rerender-dependencies             │ useEffect(() => { expr.prop }, [expr]) — deps array contains identifier X, but effect body only accesses X.prop (member        │
  │                                 │                                   │ access, not the whole object)                                                                                                  │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ High-freq event → useRef        │ rerender-use-ref-transient-values │ addEventListener('mousemove'/'scroll', e => setState(...)) inside useEffect where the handler calls a state setter             │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Transitions for scroll/input    │ rerender-transitions              │ addEventListener('scroll'/'mousemove', () => setState(...)) where setState is NOT wrapped in startTransition(...)              │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Handler in effect deps          │ advanced-event-handler-refs       │ useEffect(() => { addEventListener(e, handler) }, [handler]) where handler appears in deps — the problem is handler being a    │
  │                                 │                                   │ prop/function                                                                                                                  │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Multiple .filter on same array  │ js-combine-iterations             │ ≥2 VariableDeclarators in same block where each calls .filter(...) on the same Identifier                                      │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Repeated .find on same array    │ js-index-maps                     │ ≥2 .find(x => x.id === ...) calls on same Identifier in the same function scope                                                │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Array.includes in loop/map      │ js-set-map-lookups                │ .includes(x) calls inside .map()/.filter() callback or for-loop body where the receiver is an ArrayExpression or known array   │
  │                                 │                                   │ Identifier                                                                                                                     │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Interleaved DOM reads/writes    │ js-batch-dom-css                  │ In same block: element.style.x = y (write) followed or preceded by element.offsetWidth/.getBoundingClientRect() (read),        │
  │                                 │                                   │ interleaved                                                                                                                    │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Static import of heavy          │ bundle-dynamic-imports            │ ImportDeclaration of known heavy components (Monaco, PDF viewers, chart libs) outside of dynamic(...) call context             │
  │ component                       │                                   │                                                                                                                                │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ Derived state in useEffect      │ rerender-derived-state-no-effect  │ useEffect(() => { setX(derivedFromDeps) }, [deps]) where the setX argument is a pure expression of the deps variables (no      │
  │                                 │                                   │ async, no external calls)                                                                                                      │
  ├─────────────────────────────────┼───────────────────────────────────┼────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ useEffect init without guard    │ advanced-init-once                │ useEffect(() => { nonStateFn() }, []) — empty-dep effect calling something that is NOT a setState/dispatch — needs             │
  │                                 │                                   │ module-level let didInit guard                                                                                                 │
  └─────────────────────────────────┴───────────────────────────────────┴────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  TIER 3: Low Confidence — Requires Type Info or Semantic Flow

  These need type inference or cross-statement data flow beyond pure syntax.

  ┌──────────────────────────────────────────────────┬─────────────────────────────────────────────────────────────────────────────────────────────────────────┐
  │                       Rule                       │                                              Why it's hard                                              │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ rerender-derived-state                           │ Need to know which variables come from useState/props to identify what's "derivable during render"      │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ js-early-exit                                    │ Detecting hasError = true flag pattern requires understanding loop semantics + mutable boolean tracking │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ js-length-check-first                            │ Requires knowing that sort().join() is "expensive" — semantic                                           │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ js-cache-property-access                         │ Requires hot-path detection (inside loop body) + identifying deep property chains that repeat           │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ js-min-max-loop                                  │ Detectable in simple cases (.sort()[0] → suggest loop), but general case needs semantic intent          │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ state-context-interface                          │ Architectural pattern — cannot verify interface shape without type resolution                           │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ state-lift-state / state-decouple-implementation │ Architectural — requires understanding component tree and provider relationships                        │
  ├──────────────────────────────────────────────────┼─────────────────────────────────────────────────────────────────────────────────────────────────────────┤
  │ architecture-compound-components                 │ Intent-level architecture, not syntactically distinguishable                                            │
  └──────────────────────────────────────────────────┴─────────────────────────────────────────────────────────────────────────────────────────────────────────┘

  ---
  Summary Count

  ┌────────────────────────┬───────┬────────────────────────────────────────────────────┐
  │          Tier          │ Count │                       Notes                        │
  ├────────────────────────┼───────┼────────────────────────────────────────────────────┤
  │ Tier 1 (direct codify) │ 16    │ Ship these first — zero false-negative risk        │
  ├────────────────────────┼───────┼────────────────────────────────────────────────────┤
  │ Tier 2 (heuristic)     │ 12    │ Worth implementing; tune thresholds to avoid noise │
  ├────────────────────────┼───────┼────────────────────────────────────────────────────┤
  │ Tier 3 (semantic)      │ ~8    │ Partial detection possible for a few; skip most    │
  └────────────────────────┴───────┴────────────────────────────────────────────────────┘

  The highest-ROI rules to implement first are: rerender-no-inline-components, bundle-barrel-imports, react19-no-forwardref, rendering-conditional-render, rerender-lazy-state-init,
  rerender-memo-with-default-value, client-passive-event-listeners, and js-flatmap-filter — they're all dead-simple AST structure checks with near-zero false positive risk.

✻ Crunched for 2m 40s

❯ /usage
  ⎿  Status dialog dismissed

─────────────────────────────────────────────────────────────
