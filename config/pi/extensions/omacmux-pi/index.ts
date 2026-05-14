import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { StringEnum } from "@mariozechner/pi-ai";
import type { ExtensionAPI, ExtensionContext } from "@mariozechner/pi-coding-agent";
import { Type } from "typebox";

const omacmuxActions = [
	"status",
	"collect",
	"capture",
	"send",
	"broadcast",
	"worktrees",
	"recipes",
	"recipe_run",
	"review",
	"merge",
	"worktree_add",
	"swarm_start",
] as const;

type OmacmuxAction = (typeof omacmuxActions)[number];
type SwarmTopology = "start" | "star" | "pipe" | "pair" | "wt";
type PresetName = "scout" | "planner" | "reviewer" | "builder" | "conductor";

interface OmacmuxParams {
	action: OmacmuxAction;
	agent?: string;
	message?: string;
	target?: string;
	branch?: string;
	base?: string;
	topology?: SwarmTopology;
	count?: number;
	command?: string;
	recipe?: string;
	name?: string;
	all?: boolean;
}

interface Preset {
	description: string;
	tools: string[];
	thinkingLevel: "off" | "minimal" | "low" | "medium" | "high" | "xhigh";
	instructions: string;
}

const presets: Record<PresetName, Preset> = {
	scout: {
		description: "Fast repository reconnaissance with read-only tools plus safe shell inspection.",
		tools: ["read", "grep", "find", "ls", "bash", "omacmux"],
		thinkingLevel: "medium",
		instructions:
			"You are the scout. Map the codebase quickly, identify relevant files and commands, and return compressed context. Do not edit files. Prefer rg, git, find, ls, and targeted reads.",
	},
	planner: {
		description: "Deep planning mode for implementation strategy and risk analysis.",
		tools: ["read", "grep", "find", "ls", "bash", "omacmux"],
		thinkingLevel: "high",
		instructions:
			"You are the planner. Build a concrete implementation plan before changes. Identify files, risks, tests, and sequencing. Do not edit files unless the user explicitly switches to builder.",
	},
	reviewer: {
		description: "Read-only review mode focused on bugs, regressions, and missing tests.",
		tools: ["read", "grep", "find", "ls", "bash", "omacmux"],
		thinkingLevel: "high",
		instructions:
			"You are the reviewer. Prioritize correctness bugs, behavioral regressions, security risks, and test gaps. Keep findings specific with file and line references where possible. Do not edit files.",
	},
	builder: {
		description: "Focused implementation mode with edit/write access and verification discipline.",
		tools: ["read", "bash", "edit", "write", "grep", "find", "ls", "omacmux"],
		thinkingLevel: "high",
		instructions:
			"You are the builder. Make focused changes, keep scope tight, work in the current git worktree, and verify with the narrowest meaningful checks. Inspect files before editing.",
	},
	conductor: {
		description: "Coordination mode for tmux swarms, worktrees, recipes, and agent messaging.",
		tools: ["read", "bash", "grep", "find", "ls", "omacmux"],
		thinkingLevel: "high",
		instructions:
			"You are the conductor. Coordinate omacmux panes, swarms, worktrees, recipes, and reviews. Inspect swarm state before sending messages. Prefer delegating code changes to isolated worktrees.",
	},
};

const presetNames = Object.keys(presets) as PresetName[];

const dangerousBashPatterns: Array<{ pattern: RegExp; reason: string }> = [
	{ pattern: /\brm\s+(-[^\s]*r[^\s]*f|-[^\s]*f[^\s]*r|--recursive)/i, reason: "recursive removal" },
	{ pattern: /\bsudo\b/i, reason: "sudo escalation" },
	{ pattern: /\b(chmod|chown)\b.*\b777\b/i, reason: "broad permission change" },
	{ pattern: /\bgit\s+reset\s+--hard\b/i, reason: "destructive git reset" },
	{ pattern: /\bgit\s+clean\s+-[^\n]*[fd]/i, reason: "destructive git clean" },
	{ pattern: /\b(dd|mkfs|diskutil\s+erase|shutdown|reboot)\b/i, reason: "system or disk operation" },
	{ pattern: /\b(curl|wget)\b[^\n|;]*(\||>)[^\n]*(sh|bash)\b/i, reason: "downloaded shell execution" },
	{ pattern: /\b(gh\s+repo\s+delete|npm\s+publish)\b/i, reason: "publishing or repository deletion" },
];

const protectedBasenamePatterns = [
	/^\.env(?:\..*)?$/,
	/^.*\.pem$/,
	/^.*\.key$/,
	/^secrets?\.(json|ya?ml|toml)$/i,
	/^credentials?\.(json|ya?ml|toml)$/i,
];

function shellQuote(value: string): string {
	return `'${value.replace(/'/g, "'\\''")}'`;
}

function omacmuxRoot(): string {
	if (process.env.OMACMUX_PATH) return process.env.OMACMUX_PATH;

	const extensionDir = path.dirname(fileURLToPath(import.meta.url));
	let dir = extensionDir;
	for (let i = 0; i < 10; i += 1) {
		if (fs.existsSync(path.join(dir, "config/bash/fns/swarm"))) return dir;
		const parent = path.dirname(dir);
		if (parent === dir) break;
		dir = parent;
	}

	return path.join(process.env.HOME || "", ".local/share/omacmux");
}

function isKnownAction(action: string): action is OmacmuxAction {
	return (omacmuxActions as readonly string[]).includes(action);
}

function commandFor(params: OmacmuxParams): string {
	switch (params.action) {
		case "status":
			return 'if out=$(swarm status 2>/dev/null); then printf "%s\\n" "$out"; else swarm ls; fi';
		case "collect":
			return "swarm collect";
		case "capture":
			if (!params.agent) throw new Error("capture requires an agent id, for example agent-1");
			return `swarm capture ${shellQuote(params.agent)}`;
		case "send":
			if (!params.agent) throw new Error("send requires an agent id, for example agent-1");
			if (!params.message) throw new Error("send requires a message");
			return `swarm send ${shellQuote(params.agent)} ${shellQuote(params.message)}`;
		case "broadcast":
			if (!params.message) throw new Error("broadcast requires a message");
			return `swarm broadcast ${shellQuote(params.message)}`;
		case "worktrees":
			return "git worktree list";
		case "recipes":
			return "TMUX= recipe list";
		case "recipe_run":
			if (!params.recipe) throw new Error("recipe_run requires a recipe name");
			return `recipe ${shellQuote(params.recipe)} ${params.message ? shellQuote(params.message) : ""}`.trim();
		case "review":
			return params.target ? `review ${shellQuote(params.target)}` : "review";
		case "merge":
			return params.all ? "swarm merge --all" : "swarm merge";
		case "worktree_add":
			if (!params.branch) throw new Error("worktree_add requires a branch name");
			return params.base ? `gwa ${shellQuote(params.branch)} ${shellQuote(params.base)}` : `gwa ${shellQuote(params.branch)}`;
		case "swarm_start": {
			const topology = params.topology || "star";
			const command = params.command || "pix";
			if (topology === "pair") return `swarm pair ${shellQuote(command)}`;
			const count = params.count || (topology === "wt" ? 3 : 4);
			const name = params.name ? ` --name ${shellQuote(params.name)}` : "";
			return `swarm ${shellQuote(topology)} ${count} ${shellQuote(command)}${name}`;
		}
		default:
			throw new Error(`unknown omacmux action: ${(params as { action: string }).action}`);
	}
}

async function runOmacmux(pi: ExtensionAPI, ctx: ExtensionContext, params: OmacmuxParams, signal?: AbortSignal) {
	const root = omacmuxRoot();
	const script = [
		`export OMACMUX_PATH=${shellQuote(root)}`,
		`source ${shellQuote(path.join(root, "config/bash/fns/swarm"))} 2>/dev/null || true`,
		`source ${shellQuote(path.join(root, "config/bash/fns/recipes"))} 2>/dev/null || true`,
		`source ${shellQuote(path.join(root, "config/bash/fns/worktree"))} 2>/dev/null || true`,
		`source ${shellQuote(path.join(root, "config/bash/fns/review"))} 2>/dev/null || true`,
		commandFor(params),
	].join("\n");

	const result = await pi.exec("bash", ["-lc", script], {
		cwd: ctx.cwd,
		signal,
		timeout: 60_000,
	});

	const output = [result.stdout.trim(), result.stderr.trim() ? `stderr:\n${result.stderr.trim()}` : ""]
		.filter(Boolean)
		.join("\n\n");

	return {
		text: output || "(no output)",
		details: {
			action: params.action,
			code: result.code,
			killed: result.killed,
		},
	};
}

function parseOmacmuxCommand(args: string): OmacmuxParams {
	const parts = args.trim().split(/\s+/).filter(Boolean);
	const action = parts.shift() || "status";
	if (!isKnownAction(action)) {
		throw new Error(`unknown action "${action}". Use one of: ${omacmuxActions.join(", ")}`);
	}

	switch (action) {
		case "capture":
			return { action, agent: parts[0] };
		case "send":
			return { action, agent: parts.shift(), message: parts.join(" ") || undefined };
		case "broadcast":
			return { action, message: parts.join(" ") || undefined };
		case "recipe_run":
			return { action, recipe: parts.shift(), message: parts.join(" ") || undefined };
		case "review":
			return { action, target: parts[0] };
		case "merge":
			return { action, all: parts.includes("--all") };
		case "worktree_add":
			return { action, branch: parts[0], base: parts[1] };
		case "swarm_start":
			return {
				action,
				topology: (parts[0] as SwarmTopology | undefined) || "star",
				count: parts[1] ? Number(parts[1]) : undefined,
				command: parts[2],
				name: parts[3],
			};
		default:
			return { action };
	}
}

function normalizeToolPath(cwd: string, rawPath: unknown): string | undefined {
	if (typeof rawPath !== "string" || rawPath.trim() === "") return undefined;
	const expanded = rawPath.startsWith("~/") ? path.join(process.env.HOME || "", rawPath.slice(2)) : rawPath;
	return path.resolve(cwd, expanded);
}

function isProtectedPath(filePath: string): boolean {
	const normalized = filePath.split(path.sep).join("/");
	const basename = path.basename(normalized);
	if (protectedBasenamePatterns.some((pattern) => pattern.test(basename))) return true;

	return (
		normalized.includes("/.git/") ||
		normalized.endsWith("/.git") ||
		normalized.includes("/node_modules/") ||
		normalized.endsWith("/node_modules") ||
		normalized.includes("/.ssh/") ||
		normalized.endsWith("/.ssh") ||
		normalized.includes("/.aws/") ||
		normalized.endsWith("/.aws") ||
		normalized.includes("/.gnupg/") ||
		normalized.endsWith("/.gnupg")
	);
}

function dangerousBashReason(command: string): string | undefined {
	return dangerousBashPatterns.find(({ pattern }) => pattern.test(command))?.reason;
}

function validatePresetName(value: string | undefined): PresetName | undefined {
	if (!value) return undefined;
	return presetNames.includes(value as PresetName) ? (value as PresetName) : undefined;
}

function registerOmacmuxBridge(pi: ExtensionAPI) {
	pi.registerTool({
		name: "omacmux",
		label: "omacmux",
		description: "Inspect and operate omacmux tmux swarms, worktrees, recipes, reviews, and agent messages.",
		promptSnippet:
			"omacmux: inspect swarms, collect outputs, message agents, list/create worktrees, run recipes, launch swarms, and review branches.",
		promptGuidelines: [
			"Use omacmux status before assuming a swarm exists.",
			"Use omacmux collect or capture to read agent pane output instead of asking the user to copy it.",
			"Use omacmux send or broadcast only when the user asked you to coordinate active agents.",
			"Prefer worktree_add and swarm_start for substantial parallel code work.",
		],
		parameters: Type.Object({
			action: StringEnum(omacmuxActions, {
				description:
					"Action to run: status, collect, capture, send, broadcast, worktrees, recipes, recipe_run, review, merge, worktree_add, swarm_start.",
			}),
			agent: Type.Optional(Type.String({ description: "Agent id such as agent-1. Required for capture and send." })),
			message: Type.Optional(Type.String({ description: "Message for send, broadcast, or recipe_run args." })),
			target: Type.Optional(Type.String({ description: "Review target, such as a branch, swarm id, or agent id." })),
			branch: Type.Optional(Type.String({ description: "Branch name for worktree_add." })),
			base: Type.Optional(Type.String({ description: "Base ref for worktree_add." })),
			topology: Type.Optional(
				StringEnum(["start", "star", "pipe", "pair", "wt"] as const, {
					description: "Topology for swarm_start.",
				}),
			),
			count: Type.Optional(Type.Number({ description: "Agent count for swarm_start." })),
			command: Type.Optional(Type.String({ description: "Agent command or alias for swarm_start, default pix." })),
			recipe: Type.Optional(Type.String({ description: "Recipe name for recipe_run." })),
			name: Type.Optional(Type.String({ description: "Optional swarm name for swarm_start." })),
			all: Type.Optional(Type.Boolean({ description: "Use --all for merge." })),
		}),
		async execute(_toolCallId, params, signal, _onUpdate, ctx) {
			const parsed = params as OmacmuxParams;
			if (!isKnownAction(parsed.action)) {
				throw new Error(`unknown action "${parsed.action}". Use one of: ${omacmuxActions.join(", ")}`);
			}

			const result = await runOmacmux(pi, ctx, parsed, signal);
			return {
				content: [{ type: "text", text: result.text }],
				details: result.details,
			};
		},
	});

	pi.registerCommand("omacmux", {
		description: "Run an omacmux helper action",
		getArgumentCompletions: (prefix) =>
			omacmuxActions
				.filter((action) => action.startsWith(prefix))
				.map((action) => ({ value: action, label: action })),
		handler: async (args, ctx) => {
			const params = parseOmacmuxCommand(args);
			const result = await runOmacmux(pi, ctx, params, ctx.signal);
			if (ctx.hasUI && process.stdin.isTTY && process.stdout.isTTY) {
				await ctx.ui.editor(`omacmux ${params.action}`, result.text);
			}
		},
	});
}

function registerPresets(pi: ExtensionAPI) {
	let activePresetName: PresetName | undefined;

	function applyPreset(ctx: ExtensionContext, name: PresetName) {
		const preset = presets[name];
		const available = new Set(pi.getAllTools().map((tool) => tool.name));
		const validTools = preset.tools.filter((tool) => available.has(tool));

		pi.setActiveTools(validTools);
		pi.setThinkingLevel(preset.thinkingLevel);
		activePresetName = name;
		pi.appendEntry("omacmux-preset", { name });

		if (ctx.hasUI) {
			ctx.ui.setStatus("omacmux-preset", `omacmux:${name}`);
			ctx.ui.notify(`omacmux preset: ${name}`, "info");
		}
	}

	function clearPreset(ctx: ExtensionContext) {
		activePresetName = undefined;
		pi.appendEntry("omacmux-preset", { name: null });
		if (ctx.hasUI) {
			ctx.ui.setStatus("omacmux-preset", undefined);
			ctx.ui.notify("omacmux preset cleared", "info");
		}
	}

	pi.registerFlag("omacmux-preset", {
		description: "Start with an omacmux preset: scout, planner, reviewer, builder, conductor",
		type: "string",
	});

	pi.registerCommand("omacmux-preset", {
		description: "Apply an omacmux preset",
		getArgumentCompletions: (prefix) =>
			["clear", ...presetNames]
				.filter((name) => name.startsWith(prefix))
				.map((name) => ({ value: name, label: name })),
		handler: async (args, ctx) => {
			const requested = args.trim();
			if (requested === "clear") {
				clearPreset(ctx);
				return;
			}

			let presetName = validatePresetName(requested);
			if (!presetName && ctx.hasUI) {
				const choice = await ctx.ui.select(
					"omacmux preset",
					presetNames.map((name) => `${name} - ${presets[name].description}`),
				);
				presetName = validatePresetName(choice?.split(" ")[0]);
			}

			if (!presetName) {
				if (ctx.hasUI) ctx.ui.notify(`Unknown preset. Use: ${presetNames.join(", ")}`, "warning");
				return;
			}

			applyPreset(ctx, presetName);
		},
	});

	pi.on("session_start", async (_event, ctx) => {
		const requested = validatePresetName(pi.getFlag("omacmux-preset") as string | undefined);
		if (requested) {
			applyPreset(ctx, requested);
			return;
		}

		const lastPresetEntry = [...ctx.sessionManager.getBranch()]
			.reverse()
			.find((entry) => entry.type === "custom" && entry.customType === "omacmux-preset") as
			| { data?: { name?: PresetName | null } }
			| undefined;

		if (lastPresetEntry?.data?.name) {
			const restored = validatePresetName(lastPresetEntry.data.name);
			if (restored) applyPreset(ctx, restored);
		}
	});

	pi.on("before_agent_start", async (event) => {
		if (!activePresetName) return;
		const preset = presets[activePresetName];
		return {
			systemPrompt: `${event.systemPrompt}

## omacmux Preset: ${activePresetName}

${preset.instructions}
`,
		};
	});
}

function registerSafetyGates(pi: ExtensionAPI) {
	pi.registerFlag("omacmux-no-safety", {
		description: "Disable omacmux-pi protected path and dangerous bash gates",
		type: "boolean",
		default: false,
	});

	pi.on("tool_call", async (event, ctx) => {
		if (pi.getFlag("omacmux-no-safety") === true) return undefined;

		if (event.toolName === "write" || event.toolName === "edit") {
			const filePath = normalizeToolPath(ctx.cwd, event.input.path ?? event.input.file_path);
			if (filePath && isProtectedPath(filePath)) {
				if (ctx.hasUI) ctx.ui.notify(`Blocked write to protected path: ${filePath}`, "warning");
				return { block: true, reason: `Protected path blocked by omacmux-pi safety gate: ${filePath}` };
			}
		}

		if (event.toolName === "bash") {
			const command = event.input.command;
			if (typeof command !== "string") return undefined;

			const reason = dangerousBashReason(command);
			if (!reason) return undefined;

			if (!ctx.hasUI) {
				return { block: true, reason: `Dangerous bash command blocked (${reason}); no UI for confirmation.` };
			}

			const choice = await ctx.ui.select(`Dangerous bash command (${reason}):\n\n${command}\n\nAllow?`, [
				"Allow once",
				"Block",
			]);

			if (choice !== "Allow once") {
				return { block: true, reason: `Dangerous bash command blocked (${reason}).` };
			}
		}

		return undefined;
	});
}

export default function omacmuxPi(pi: ExtensionAPI) {
	registerOmacmuxBridge(pi);
	registerPresets(pi);
	registerSafetyGates(pi);
}
