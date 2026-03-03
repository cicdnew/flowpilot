/**
 * ECC Custom Tool: Format Code
 *
 * Language-aware code formatter that auto-detects the project's formatter.
 * Supports: Biome/Prettier (JS/TS), Black (Python), gofmt (Go), rustfmt (Rust)
 */

import { tool } from "@opencode-ai/plugin/tool"

export default tool({
  description: "Format a file using the project's configured formatter. Auto-detects Biome, Prettier, Black, gofmt, or rustfmt.",
  args: {
    filePath: tool.schema.string().describe("Path to the file to format"),
    formatter: tool.schema.string().optional().describe("Override formatter: biome, prettier, black, gofmt, rustfmt (default: auto-detect)"),
  },
  async execute(args, context) {
    const { filePath, formatter } = args
    const ext = filePath.split(".").pop()?.toLowerCase() || ""

    let detected = formatter
    if (!detected) {
      if (["ts", "tsx", "js", "jsx", "json", "css", "scss"].includes(ext)) {
        detected = "prettier"
      } else if (["py", "pyi"].includes(ext)) {
        detected = "black"
      } else if (ext === "go") {
        detected = "gofmt"
      } else if (ext === "rs") {
        detected = "rustfmt"
      }
    }

    if (!detected) {
      return JSON.stringify({ formatted: false, message: `No formatter detected for .${ext} files` })
    }

    const commands: Record<string, string> = {
      biome: `npx @biomejs/biome format --write ${filePath}`,
      prettier: `npx prettier --write ${filePath}`,
      black: `black ${filePath}`,
      gofmt: `gofmt -w ${filePath}`,
      rustfmt: `rustfmt ${filePath}`,
    }

    const cmd = commands[detected]
    if (!cmd) {
      return JSON.stringify({ formatted: false, message: `Unknown formatter: ${detected}` })
    }

    return JSON.stringify({
      command: cmd,
      formatter: detected,
      filePath,
      instructions: `Run this command to format:\n\n${cmd}`,
    })
  },
})
