#!/usr/bin/env bash
# based on http://stackoverflow.com/a/3464399/1383268
# assumes that the hooks-wrapper script is located at <repo-root>/bin/git/hooks-wrapper

HOOKS_DIR="$(git rev-parse --show-toplevel)/.git/hooks"

hook="pre-commit"

# If the hook already exists, is a file, and is not a symlink
if [[ ! -h ${HOOKS_DIR}/${hook} ]] && [[ -f ${HOOKS_DIR}/${hook} ]]; then
    mv "${HOOKS_DIR}/${hook}" "${HOOKS_DIR}/${hook}.local"
fi

# symlink our wrapper to the pre-commit hook
ln -s -f ../../bin/git/hooks-wrapper.bash "${HOOKS_DIR}/${hook}"
