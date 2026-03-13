-- Telescope worktree picker: <leader>gw to switch nvim's cwd to a worktree
return {
  {
    "nvim-telescope/telescope.nvim",
    keys = {
      {
        "<leader>gw",
        function()
          local pickers = require("telescope.pickers")
          local finders = require("telescope.finders")
          local conf = require("telescope.config").values
          local actions = require("telescope.actions")
          local action_state = require("telescope.actions.state")

          local handle = io.popen("git worktree list 2>/dev/null")
          if not handle then
            vim.notify("Not in a git repo", vim.log.levels.WARN)
            return
          end
          local result = handle:read("*a")
          handle:close()

          local worktrees = {}
          for line in result:gmatch("[^\n]+") do
            if line ~= "" then
              table.insert(worktrees, line)
            end
          end

          if #worktrees == 0 then
            vim.notify("No worktrees found", vim.log.levels.WARN)
            return
          end

          pickers
            .new({}, {
              prompt_title = "Git Worktrees",
              finder = finders.new_table({ results = worktrees }),
              sorter = conf.generic_sorter({}),
              attach_mappings = function(prompt_bufnr)
                actions.select_default:replace(function()
                  local selection = action_state.get_selected_entry()
                  actions.close(prompt_bufnr)
                  local path = selection.value:match("^(%S+)")
                  vim.cmd("cd " .. vim.fn.fnameescape(path))
                  vim.notify("Switched to: " .. path)
                end)
                return true
              end,
            })
            :find()
        end,
        desc = "Switch worktree",
      },
    },
  },
}
