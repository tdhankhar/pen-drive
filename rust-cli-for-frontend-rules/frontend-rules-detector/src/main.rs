use anyhow::Result;
use clap::Parser;
use oxc_allocator::Allocator;
use oxc_ast::ast::{Expression, LogicalExpression, LogicalOperator};
use oxc_ast_visit::Visit;
use oxc_parser::Parser as OxcParser;
use oxc_span::{GetSpan, SourceType};
use std::fs;
use std::path::{Path, PathBuf};
use walkdir::WalkDir;

#[derive(Parser)]
#[command(name = "frontend-rules-detector")]
#[command(about = "Detect frontend code violations")]
struct Args {
    /// Path to analyze
    path: PathBuf,

    /// Rules to check (comma-separated)
    #[arg(short, long, default_value = "rendering-conditional-render,js-flatmap-filter")]
    rules: String,
}

#[derive(Debug, Clone)]
struct Violation {
    rule: String,
    file: String,
    line: u32,
    column: u32,
    message: String,
}

fn main() -> Result<()> {
    let args = Args::parse();
    let mut violations = Vec::new();

    for entry in WalkDir::new(&args.path)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| {
            e.path()
                .extension()
                .and_then(|ext| ext.to_str())
                .map(|ext| ext == "tsx" || ext == "ts" || ext == "jsx" || ext == "js")
                .unwrap_or(false)
        })
    {
        let path = entry.path();
        if let Ok(content) = fs::read_to_string(path) {
            if args.rules.contains("rendering-conditional-render") {
                check_conditional_render_ast(&mut violations, path, &content);
            }
            if args.rules.contains("js-flatmap-filter") {
                check_flatmap_filter(&mut violations, path, &content);
            }
        }
    }

    // Output results
    if violations.is_empty() {
        println!("✓ No violations found");
    } else {
        println!("Found {} violations:\n", violations.len());

        let mut by_rule = std::collections::BTreeMap::new();
        for v in violations {
            by_rule
                .entry(v.rule.clone())
                .or_insert_with(Vec::new)
                .push(v);
        }

        for (rule, items) in by_rule {
            println!("  {}  [{}]", rule.to_uppercase(), items.len());
            for item in items {
                println!(
                    "    {}:{}:{} – {}",
                    item.file, item.line, item.column, item.message
                );
            }
            println!();
        }
    }

    Ok(())
}

fn check_conditional_render_ast(
    violations: &mut Vec<Violation>,
    path: &Path,
    content: &str,
) {
    let source_type = SourceType::from_path(path).unwrap_or_default();
    let allocator = Allocator::default();
    let parser = OxcParser::new(&allocator, content, source_type);
    let ret = parser.parse();
    let program = ret.program;

    let mut visitor = ConditionalRenderVisitor {
        violations,
        file_path: path.display().to_string(),
        source: content,
    };

    visitor.visit_program(&program);
}

struct ConditionalRenderVisitor<'a> {
    violations: &'a mut Vec<Violation>,
    file_path: String,
    source: &'a str,
}

impl<'ast> Visit<'ast> for ConditionalRenderVisitor<'_> {
    fn visit_logical_expression(&mut self, expr: &LogicalExpression<'ast>) {
        // Check for && operator in conditional render context
        if expr.operator == LogicalOperator::And {
            // Check if right side is JSX - extract text from the AST span
            let right_start = expr.right.span().start as usize;
            let right_end = expr.right.span().end as usize;
            
            if right_start < self.source.len() && right_end <= self.source.len() {
                let right_str = self.source[right_start..right_end].trim();
                
                // Check if this is JSX (right side starts with <)
                if right_str.starts_with('<') {
                    // Evaluate left side - check if it's a safe condition
                    if !is_safe_condition(&expr.left) {
                        // Extract left side text for error message
                        let left_start = expr.left.span().start as usize;
                        let left_end = expr.left.span().end as usize;
                        let left_str = if left_start < self.source.len() && left_end <= self.source.len() {
                            self.source[left_start..left_end].trim().to_string()
                        } else {
                            "condition".to_string()
                        };

                        let (line, col) = get_line_col(expr.span.start, self.source);
                        self.violations.push(Violation {
                            rule: "rendering-conditional-render".to_string(),
                            file: self.file_path.clone(),
                            line,
                            column: col,
                            message: format!(
                                "Unsafe && with '{}' – use ternary or explicit boolean check",
                                left_str
                            ),
                        });
                    }
                }
            }
        }

        oxc_ast_visit::walk::walk_logical_expression(self, expr);
    }
}

fn is_safe_condition(expr: &Expression) -> bool {
    match expr {
        // Safe: boolean identifiers
        Expression::Identifier(id) => is_boolean_name(&id.name),

        // Safe: comparisons
        Expression::BinaryExpression(bin) => {
            use oxc_syntax::operator::BinaryOperator;
            matches!(
                bin.operator,
                BinaryOperator::StrictEquality
                    | BinaryOperator::StrictInequality
                    | BinaryOperator::Equality
                    | BinaryOperator::Inequality
                    | BinaryOperator::GreaterThan
                    | BinaryOperator::LessThan
                    | BinaryOperator::GreaterEqualThan
                    | BinaryOperator::LessEqualThan
            )
        }

        // Safe: negation
        Expression::UnaryExpression(unary) => {
            use oxc_syntax::operator::UnaryOperator;
            matches!(unary.operator, UnaryOperator::LogicalNot)
        }

        // Safe: optional chaining
        Expression::ChainExpression(_) => true,

        // Check member expressions
        Expression::StaticMemberExpression(static_mem) => is_boolean_name(&static_mem.property.name),
        Expression::ComputedMemberExpression(_) => false,

        _ => false,
    }
}

fn is_boolean_name(name: &str) -> bool {
    let prefixes = vec![
        "is", "has", "can", "should", "will", "show", "hide", "enable", "disable", "no",
    ];

    let suffixes = vec![
        "enabled", "disabled", "active", "inactive", "visible", "hidden", "open", "closed",
        "loading", "pending", "ready", "error", "success", "valid", "invalid",
    ];

    // Check prefixes - must have capital letter after prefix
    for p in &prefixes {
        if name.starts_with(p) && name.len() > p.len() {
            if let Some(next_char) = name.chars().nth(p.len()) {
                if next_char.is_uppercase() {
                    return true;
                }
            }
        }
    }

    // Check suffixes (case-insensitive)
    // Must be at least prefix + suffix length to avoid single-word matches
    let name_lower = name.to_lowercase();
    for suffix in &suffixes {
        if name_lower.ends_with(suffix) {
            // Ensure there's something before the suffix (at least 2 chars)
            if name_lower.len() > suffix.len() {
                return true;
            }
        }
    }

    false
}

fn get_line_col(offset: u32, content: &str) -> (u32, u32) {
    let mut line = 1u32;
    let mut col = 0u32;

    for (i, ch) in content.chars().enumerate() {
        if i >= offset as usize {
            break;
        }
        if ch == '\n' {
            line += 1;
            col = 0;
        } else {
            col += 1;
        }
    }

    (line, col)
}

fn check_flatmap_filter(
    violations: &mut Vec<Violation>,
    path: &Path,
    content: &str,
) {
    let lines: Vec<&str> = content.lines().collect();

    for (line_num, line) in lines.iter().enumerate() {
        let line_no = (line_num + 1) as u32;

        if let Some(filter_pos) = line.find(".filter(") {
            if let Some(_map_pos) = line[..filter_pos].rfind(".map(") {
                let after_filter = &line[filter_pos + 8..];
                let filter_arg = extract_until_paren_close(after_filter);

                if is_flatmap_filter_arg(&filter_arg) {
                    let col = filter_pos as u32;
                    violations.push(Violation {
                        rule: "js-flatmap-filter".to_string(),
                        file: path.display().to_string(),
                        line: line_no,
                        column: col,
                        message: "Use .flatMap() instead of .map().filter() – more efficient".to_string(),
                    });
                }
            }
        }
    }
}

fn extract_until_paren_close(s: &str) -> String {
    let mut depth = 0;
    for (i, ch) in s.chars().enumerate() {
        match ch {
            '(' => depth += 1,
            ')' => {
                if depth == 0 {
                    return s[..i].to_string();
                }
                depth -= 1;
            }
            _ => {}
        }
    }
    s.to_string()
}

fn is_flatmap_filter_arg(arg: &str) -> bool {
    let arg = arg.trim();
    arg == "Boolean"
        || arg == "x => x"
        || arg == "item => item"
        || arg == "el => el"
        || arg == "v => v"
        || arg == "a => a"
}
