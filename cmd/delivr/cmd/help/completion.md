Generate shell completion scripts for delivr.

## Usage

```bash
delivr completion [bash|zsh|fish|powershell]
```

## Setup

### Bash

```bash
source <(delivr completion bash)
# Or add to ~/.bashrc:
echo 'source <(delivr completion bash)' >> ~/.bashrc
```

### Zsh

```bash
delivr completion zsh > "${fpath[1]}/_delivr"
# Then restart your shell
```

### Fish

```bash
delivr completion fish | source
# Or persist:
delivr completion fish > ~/.config/fish/completions/delivr.fish
```
