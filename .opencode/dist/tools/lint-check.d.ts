/**
 * ECC Custom Tool: Lint Check
 *
 * Multi-language linter that auto-detects the project's linting tool.
 * Supports: ESLint/Biome (JS/TS), Pylint/Ruff (Python), golangci-lint (Go)
 */
declare const _default: {
    description: string;
    args: {
        target: import("zod").ZodOptional<import("zod").ZodString>;
        fix: import("zod").ZodOptional<import("zod").ZodBoolean>;
        linter: import("zod").ZodOptional<import("zod").ZodString>;
    };
    execute(args: {
        target?: string | undefined;
        fix?: boolean | undefined;
        linter?: string | undefined;
    }, context: import("@opencode-ai/plugin/tool").ToolContext): Promise<string>;
};
export default _default;
//# sourceMappingURL=lint-check.d.ts.map