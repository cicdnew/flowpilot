/**
 * ECC Custom Tool: Git Summary
 *
 * Provides a comprehensive git status including branch info, status,
 * recent log, and diff against base branch.
 */
import { tool } from "@opencode-ai/plugin/tool";
export default tool({
    description: "Get comprehensive git summary: branch, status, recent log, and diff against base branch.",
    args: {
        depth: tool.schema.number().optional().describe("Number of recent commits to show (default: 5)"),
        includeDiff: tool.schema.boolean().optional().describe("Include diff against base branch (default: true)"),
        baseBranch: tool.schema.string().optional().describe("Base branch for comparison (default: main)"),
    },
    async execute(args, context) {
        const depth = args.depth ?? 5;
        const includeDiff = args.includeDiff ?? true;
        const baseBranch = args.baseBranch ?? "main";
        const commands = [
            `git branch --show-current`,
            `git status --short`,
            `git log --oneline -${depth}`,
        ];
        if (includeDiff) {
            commands.push(`git diff --cached --stat`);
            commands.push(`git diff ${baseBranch}...HEAD --stat`);
        }
        return JSON.stringify({
            commands,
            instructions: `Run these git commands to get a comprehensive summary:\n\n${commands.join("\n")}`,
        });
    },
});
//# sourceMappingURL=git-summary.js.map