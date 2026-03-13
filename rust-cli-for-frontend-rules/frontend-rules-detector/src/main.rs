use anyhow::Result;
use clap::Parser;
use oxc_allocator::Allocator;
use oxc_ast::ast::Statement;
use oxc_parser::Parser as OxcParser;
use oxc_span::SourceType;
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
                check_conditional_render(&mut violations, path, &content);
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

        // Group by rule
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

fn check_conditional_render(
    violations: &mut Vec<Violation>,
    path: &Path,
    content: &str,
) {
    let lines: Vec<&str> = content.lines().collect();

    for (line_num, line) in lines.iter().enumerate() {
        let line_no = (line_num + 1) as u32;

        // Look for JSX expression with && pattern
        if let Some(brace_pos) = line.find('{') {
            // Find the closing brace
            if let Some(close_brace) = line[brace_pos + 1..].find('}') {
                let expr = &line[brace_pos + 1..brace_pos + 1 + close_brace];

                // Check for && operator
                if let Some(and_pos) = expr.find("&&") {
                    let before_and = expr[..and_pos].trim();
                    let after_and = expr[and_pos + 2..].trim();

                    // Must have JSX after && (look for < character)
                    if after_and.starts_with('<') {
                        // Check if the condition is unsafe
                        if is_unsafe_condition(before_and) {
                            let col = (brace_pos + 1 + and_pos) as u32;
                            violations.push(Violation {
                                rule: "rendering-conditional-render".to_string(),
                                file: path.display().to_string(),
                                line: line_no,
                                column: col,
                                message: format!("Unsafe && with '{}' – use ternary or explicit boolean check", before_and),
                            });
                        }
                    }
                }
            }
        }
    }
}

fn is_unsafe_condition(condition: &str) -> bool {
    let trimmed = condition.trim();

    // SAFE: Boolean flags (is*, has*, can*, should*, will*, show*, hide*, enable*, disable*)
    if is_boolean_identifier(trimmed) {
        return false;
    }

    // SAFE: Comparison operators
    if trimmed.contains('>') || trimmed.contains('<') || trimmed.contains('=') {
        return false;
    }

    // SAFE: Negation (but not double negation - those need !! which is different)
    if trimmed.starts_with('!') && !trimmed.starts_with("!!") {
        return false;
    }

    // SAFE: Double negation !!
    if trimmed.starts_with("!!") {
        return false;
    }

    // SAFE: Optional chaining
    if trimmed.contains("?.") {
        return false;
    }

    // SAFE: Method calls with .
    if trimmed.contains("?.") || trimmed.ends_with(')') && trimmed.contains('(') {
        return false;
    }

    // UNSAFE: Bare identifier/property that could be falsy number/string/array
    true
}

fn is_boolean_identifier(s: &str) -> bool {
    let s_lower = s.to_lowercase();
    let prefixes = vec![
        "is", "has", "can", "should", "will", "show", "hide", "enable", "disable",
    ];
    prefixes.iter().any(|p| s.starts_with(p) && s.len() > p.len() && s.chars().nth(p.len()).unwrap().is_uppercase())
}

fn check_flatmap_filter(
    violations: &mut Vec<Violation>,
    path: &Path,
    content: &str,
) {
    let lines: Vec<&str> = content.lines().collect();

    for (line_num, line) in lines.iter().enumerate() {
        let line_no = (line_num + 1) as u32;

        // Pattern: .map(...).filter(Boolean) or .map(...).filter(x => x)
        if let Some(filter_pos) = line.find(".filter(") {
            // Check if there's .map( before this
            if let Some(map_pos) = line[..filter_pos].rfind(".map(") {
                // Extract filter argument
                let after_filter = &line[filter_pos + 8..]; // ".filter(" is 8 chars
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
    // Pattern: Boolean (the constructor) or x => x (identity function)
    arg == "Boolean"
        || arg == "x => x"
        || arg == "item => item"
        || arg == "el => el"
        || arg == "v => v"
        || arg == "a => a"
        || arg.matches("=>").count() == 1
            && arg.split("=>").last().map(|p| p.trim()) == Some("x")
}
