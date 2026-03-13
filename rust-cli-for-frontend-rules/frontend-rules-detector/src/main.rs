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
    #[arg(short, long, default_value = "barrel-imports,conditional-render")]
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
            let source_type = SourceType::from_path(path).unwrap_or_default();
            let allocator = Allocator::default();
            let parser = OxcParser::new(&allocator, &content, source_type);
            let result = parser.parse();

            let program = result.program;

            if args.rules.contains("barrel-imports") {
                check_barrel_imports(&mut violations, path, &program, &content);
            }
            if args.rules.contains("conditional-render") {
                check_conditional_render(&mut violations, path, &content);
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

fn check_barrel_imports(
    violations: &mut Vec<Violation>,
    path: &Path,
    program: &oxc_ast::ast::Program<'_>,
    content: &str,
) {
    let barrel_packages = vec![
        "lucide-react",
        "@mui/material",
        "@mui/icons-material",
        "@tabler/icons-react",
        "react-icons",
        "lodash",
        "date-fns",
    ];

    for stmt in &program.body {
        if let Statement::ImportDeclaration(import) = stmt {
            let source = import.source.value.to_string();

            if barrel_packages.iter().any(|pkg| source.contains(pkg)) {
                if let Some(specifiers) = &import.specifiers {
                    if !specifiers.is_empty() {
                        // Check for named imports
                        for spec in specifiers {
                            use oxc_ast::ast::ImportDeclarationSpecifier;
                            if !matches!(spec, ImportDeclarationSpecifier::ImportDefaultSpecifier(_)) {
                                let (line, col) = get_line_col(import.span.start, content);
                                violations.push(Violation {
                                    rule: "barrel-imports".to_string(),
                                    file: path.display().to_string(),
                                    line,
                                    column: col,
                                    message: format!("Named imports from '{}'", source),
                                });
                                break; // Only report once per import
                            }
                        }
                    }
                }
            }
        }
    }
}

fn check_conditional_render(
    violations: &mut Vec<Violation>,
    path: &Path,
    content: &str,
) {
    // Simple regex-like search for && in JSX context
    let lines: Vec<&str> = content.lines().collect();

    for (line_num, line) in lines.iter().enumerate() {
        let line_no = (line_num + 1) as u32;

        // Look for { ... && pattern
        if let Some(brace_pos) = line.find('{') {
            if let Some(and_pos) = line[brace_pos..].find("&&") {
                let and_pos = brace_pos + and_pos;
                let before = &line[brace_pos + 1..and_pos].trim();

                // Skip if it's a safe boolean check
                let is_safe = before.is_empty()
                    || before.ends_with('>') // comparison
                    || before.ends_with('=')
                    || before.ends_with('!')
                    || before.starts_with("is")
                    || before.starts_with("has");

                if !is_safe {
                    violations.push(Violation {
                        rule: "conditional-render".to_string(),
                        file: path.display().to_string(),
                        line: line_no,
                        column: and_pos as u32,
                        message: "Unsafe && in JSX – use ternary or check for falsy values".to_string(),
                    });
                }
            }
        }
    }
}
