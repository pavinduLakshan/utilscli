# Utils CLI

![Utils CLI](utils-cli.png)

Utils CLI (`uc`) brings some day-today dev utilities to the terminal as a small Go binary. Run `uc --help` for the full command list.

![uc api — a Postman-style terminal API client](utils-cli-api-client.png)

## Install

Install the latest release with one command:

```sh
curl -fsSL https://raw.githubusercontent.com/pavinduLakshan/utilscli/main/install.sh | sh
```

## Usage

Run `uc` with no arguments to open the interactive terminal UI. UUID and password generation do not require input.

### Interactive controls

- **Up/Down:** Choose a tool, move through the input, or scroll the output when its pane is active.
- **Tab:** Move focus between the tool list, input editor, and output pane.
- **Enter:** Add a line in the input editor, or run a generator that needs no input.
- **Ctrl+R:** Run the selected tool.
- **Page Up/Page Down:** Scroll long output by a page from any pane. The mouse wheel also scrolls output.
- **Home/End:** Jump to the beginning or end of the output while its pane is active.
- **Ctrl+Y:** Copy the complete output to the system clipboard. A green **Output copied** notification confirms success; a yellow message explains empty-output or clipboard fallback cases.
- **Esc:** Exit the interactive UI.

Individual commands are also supported. See the documentation for all supported commands.

Eg:

```sh
uc b64-encode "I'm feeling Lucky"
# SSdtIGZlZWxpbmcgTHVja3k=
```
