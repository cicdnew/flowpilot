/**
 * Everything Claude Code (ECC) Plugin for OpenCode
 *
 * This package provides a complete OpenCode plugin with:
 * - 13 specialized agents (planner, architect, code-reviewer, etc.)
 * - 31 commands (/plan, /tdd, /code-review, etc.)
 * - Plugin hooks (auto-format, TypeScript check, console.log warning, env injection, etc.)
 * - Custom tools (run-tests, check-coverage, security-audit, format-code, lint-check, git-summary)
 * - 37 skills (coding-standards, security-review, tdd-workflow, etc.)
 *
 * Usage:
 *
 * Option 1: Install via npm
 * ```bash
 * npm install ecc-universal
 * ```
 *
 * Then add to your opencode.json:
 * ```json
 * {
 *   "plugin": ["ecc-universal"]
 * }
 * ```
 *
 * Option 2: Clone and use directly
 * ```bash
 * git clone https://github.com/affaan-m/everything-claude-code
 * cd everything-claude-code
 * opencode
 * ```
 *
 * @packageDocumentation
 */
export { ECCHooksPlugin, default } from "./plugins/index.js";
export * from "./plugins/index.js";
export declare const VERSION = "1.6.0";
export declare const metadata: {
    name: string;
    version: string;
    description: string;
    author: string;
    features: {
        agents: number;
        commands: number;
        skills: number;
        hookEvents: string[];
        customTools: string[];
    };
};
//# sourceMappingURL=index.d.ts.map