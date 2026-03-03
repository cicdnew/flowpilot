/**
 * ECC Custom Tool: Format Code
 *
 * Language-aware code formatter that auto-detects the project's formatter.
 * Supports: Biome/Prettier (JS/TS), Black (Python), gofmt (Go), rustfmt (Rust)
 */
declare const _default: {
    description: string;
    args: {
        filePath: import("zod").ZodString;
        formatter: import("zod").ZodOptional<import("zod").ZodString>;
    };
    execute(args: {
        filePath: string;
        formatter?: string | undefined;
    }, context: import("@opencode-ai/plugin/tool").ToolContext): Promise<string>;
};
export default _default;
//# sourceMappingURL=format-code.d.ts.map