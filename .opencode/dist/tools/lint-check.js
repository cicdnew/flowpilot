/**
 * ECC Custom Tool: Lint Check
 *
 * Multi-language linter that auto-detects the project's linting tool.
 * Supports: ESLint/Biome (JS/TS), Pylint/Ruff (Python), golangci-lint (Go)
 */
import { tool } from "@opencode-ai/plugin/tool";
export default tool({
    description: "Run linter on files or directories. Auto-detects ESLint, Biome, Ruff, Pylint, or golangci-lint.",
    args: {
        target: tool.schema.string().optional().describe("File or directory to lint (default: current directory)"),
        fix: tool.schema.boolean().optional().describe("Auto-fix issues if supported (default: false)"),
        linter: tool.schema.string().optional().describe("Override linter: eslint, biome, ruff, pylint, golangci-lint (default: auto-detect)"),
    },
    async execute(args, context) {
        const target = args.target ?? ".";
        const fix = args.fix ?? false;
        const linter = args.linter ?? "eslint";
        const fixFlag = fix ? " --fix" : "";
        const commands = {
            biome: `npx @biomejs/biome lint${fix ? " --write" : ""} ${target}`,
            eslint: `npx eslint${fixFlag} ${target}`,
            ruff: `ruff check${fixFlag} ${target}`,
            pylint: `pylint ${target}`,
            "golangci-lint": `golangci-lint run${fixFlag} ${target}`,
        };
        const cmd = commands[linter];
        if (!cmd) {
            return JSON.stringify({ success: false, message: `Unknown linter: ${linter}` });
        }
        return JSON.stringify({
            command: cmd,
            linter,
            target,
            fix,
            instructions: `Run this command to lint:\n\n${cmd}`,
        });
    },
});
//# sourceMappingURL=lint-check.js.map