# uc

`uc` brings the browser utilities from [WSO2 CS Tools](https://wso2-cs.github.io/cs-tools/) to the terminal as a small Go binary.

Install the latest release with one command:

```sh
curl -fsSL https://raw.githubusercontent.com/pavinduLakshan/utilscli/main/install.sh | sh
```

The installer downloads the matching release binary, installs it to
`~/.local/bin`, and adds that folder to your zsh/bash/POSIX shell startup file
when needed. Open a new terminal, then:

```sh
uc b64-encode osidosodi
# b3NpZG9zb2Rp
```

Set `UC_BIN_DIR` to install somewhere else, e.g. `UC_BIN_DIR=$HOME/bin sh install.sh`.
Set `UC_REPO` if you publish the project under a different GitHub repository.

It supports the documented encoders, JSON/XML formatters, JWT and SAML decoding, hashes, UUIDs, passwords, and timestamps. Run `uc --help` for the full command list.

Run `uc` with no arguments to open the interactive terminal UI. The tools panel is active first: use the arrow keys to choose a tool, then press Tab to focus that tool's editor. Enter adds a line to an active editor; Ctrl+R runs the selected tool. UUID and password generation do not require input. Use `uc --help` for the complete command list.
