-- Autocmds are automatically loaded on the VeryLazy event
-- Default autocmds that are always set: https://github.com/LazyVim/LazyVim/blob/main/lua/lazyvim/config/autocmds.lua
--
-- Add any additional autocmds here
-- with `vim.api.nvim_create_autocmd`
--
-- Or remove existing autocmds by their group name (which is prefixed with `lazyvim_` for the defaults)
-- e.g. vim.api.nvim_del_augroup_by_name("lazyvim_wrap_spell")

-- Open PDFs in macOS Preview instead of displaying binary in buffer
vim.api.nvim_create_autocmd("BufReadCmd", {
  group = vim.api.nvim_create_augroup("pdf_open_external", { clear = true }),
  pattern = "*.pdf",
  callback = function(ev)
    vim.fn.jobstart({ "open", vim.api.nvim_buf_get_name(ev.buf) }, { detach = true })
    vim.defer_fn(function()
      vim.api.nvim_buf_delete(ev.buf, { force = true })
    end, 100)
  end,
})

-- Reactive worktree pane focus: when nvim's cwd changes, focus the tmux agent
-- pane that corresponds to that worktree (set by twdl via @wt_panes window var)
vim.api.nvim_create_autocmd("DirChanged", {
  group = vim.api.nvim_create_augroup("worktree_pane_focus", { clear = true }),
  callback = function()
    local cwd = vim.fn.getcwd()
    local ok, pane_map = pcall(function()
      return vim.fn.system("tmux show-option -wqv @wt_panes 2>/dev/null"):gsub("%s+$", "")
    end)
    if not ok or pane_map == "" then
      return
    end
    for entry in pane_map:gmatch("[^,]+") do
      local pane_id, path = entry:match("([^:]+):(.*)")
      if pane_id and path and cwd:find(path, 1, true) then
        vim.fn.system("tmux select-pane -t " .. pane_id)
        break
      end
    end
  end,
})
