# uc

`uc` brings the browser utilities from [WSO2 CS Tools](https://wso2-cs.github.io/cs-tools/) to the terminal as a small, dependency-free Go binary.

Install the latest release with one command:

```sh
curl -fsSL https://raw.githubusercontent.com/pavinduLakshan/utilscli/main/install.sh | sh
```

The installer downloads the matching release binary, installs it to
`~/.local/bin`, and adds that folder to your zsh/bash/POSIX shell startup file
when needed. Open a new terminal, then:

```sh
uc 'base64 osidosodi'
# b3NpZG9zb2Rp
```

Set `UC_BIN_DIR` to install somewhere else, e.g. `UC_BIN_DIR=$HOME/bin sh install.sh`.
Set `UC_REPO` if you publish the project under a different GitHub repository.

It supports the documented encoders, JSON/XML formatters, JWT and SAML decoding, hashes, UUIDs, passwords, timestamps, regex matching, diff, HTTP status lookup, and CORS headers. Run `uc --help` for the full command list.

Run `uc` with no arguments for a natural-language prompt. It accepts multiple lines; enter a blank line to submit. Run `uc --help` to list the available tools.

Natural requests are routed locally whenever possible. For ambiguous requests, `uc` invokes your locally authenticated [Claude Code](https://docs.anthropic.com/en/docs/claude-code/cli-usage) client in non-interactive JSON mode. Install Claude Code and log in with your subscription first; `uc` does not need an API key or model setting. Claude Code receives only the ambiguous request, and `uc` only accepts an allow-listed local utility choice in return.
