#!/bin/sh
# check-push.sh: run before EVERY push. Fails if the commits you are about to
# push (BASE..HEAD, default origin/main..HEAD) add:
#   - a file named core or core.* (a core dump holds the process environment)
#   - a file over 1 MiB (unless listed in tools/check-push.allow)
#   - a line that sets a credential-looking variable to a non-empty value
#   - a private-key header or a well-known token shape
#   - the VALUE of a credential-looking variable of the current environment
# It never prints a secret value, only the rule and the file (or variable name).
# POSIX shell, no packages needed beyond git.
#
# Usage: tools/check-push.sh            (BASE=origin/main)
#        CHECK_PUSH_BASE=origin/foo tools/check-push.sh
#        CHECK_PUSH_ALLOW=path/to/allowfile tools/check-push.sh

set -u
ulimit -c 0 2>/dev/null

base=${CHECK_PUSH_BASE:-origin/main}
top=$(git rev-parse --show-toplevel 2>/dev/null) || { echo "check-push: not in a git repository" >&2; exit 2; }
allow=${CHECK_PUSH_ALLOW:-$top/tools/check-push.allow}
maxsize=1048576

git rev-parse --verify -q "$base^{commit}" >/dev/null || {
	echo "check-push: base $base not found (run git fetch origin)" >&2
	exit 2
}
range="$base..HEAD"
fail=0
tmp=$(mktemp -d "${TMPDIR:-/tmp}/check-push.XXXXXX") || exit 2
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

complain() { # rule, where
	echo "check-push: FAIL [$1] $2" >&2
	fail=1
}

# 1 and 2: every object the range adds (this also catches a file that is
# added in one commit and deleted in the next: it would still be pushed).
git rev-list --objects "$range" >"$tmp/objs"
git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' <"$tmp/objs" |
	while read -r type sha size rest; do
		[ "$type" = blob ] || continue
		name=${rest##*/}
		case $name in
		core | core.* | *.core)
			echo "core-file $rest"
			;;
		esac
		if [ "$size" -gt "$maxsize" ]; then
			if ! { [ -f "$allow" ] && grep -Fxq -- "$rest" "$allow"; }; then
				echo "big-file $rest"
			fi
		fi
	done >"$tmp/files"
while read -r rule path; do
	case $rule in
	core-file) complain "core file (could hold the environment)" "$path" ;;
	big-file) complain "file over 1 MiB, not in tools/check-push.allow" "$path" ;;
	esac
done <"$tmp/files"

# 3: added lines of the whole range.
git log -p --no-color --format='commit %H' --no-ext-diff "$range" >"$tmp/patch"
awk '
	/^\+\+\+ b\// { file = substr($0, 7); next }
	/^\+\+\+ /    { next }
	/^\+/         { print file "\t" substr($0, 2) }
' "$tmp/patch" >"$tmp/added"

# 3a: name = non-empty value. A value that starts with $ < { ( or ${ is a
# reference, not a secret. Empty values and placeholders in <angle> are fine.
credname='(AWS_SECRET_ACCESS_KEY|AWS_ACCESS_KEY_ID|AWS_SESSION_TOKEN|GITHUB_TOKEN|GH_TOKEN|ANTHROPIC_API_KEY|[A-Za-z0-9_]*_(TOKEN|SECRET|PASSWORD|KEY))'
# Shell expansions such as ${NAME:+yes} are references, not values: drop them first.
sed -E 's/\$\{[^}]*\}//g' "$tmp/added" |
	grep -E "(^|[^A-Za-z0-9_])$credname[\"']?[[:space:]]*[=:][[:space:]]*[\"']?[^[:space:]\"'\$<{(]" |
	cut -f1 | sort -u |
	while read -r f; do echo "$f"; done >"$tmp/namehits"
while read -r f; do
	complain "credential-looking variable set to a value" "$f"
done <"$tmp/namehits"

# 3b: private keys and well-known token shapes.
grep -E -e '-----BEGIN [A-Z ]*PRIVATE KEY-----' "$tmp/added" | cut -f1 | sort -u >"$tmp/keyhits"
while read -r f; do complain "private key header" "$f"; done <"$tmp/keyhits"
grep -E -e '(ghp_|gho_|ghs_|github_pat_)[A-Za-z0-9_]{20,}' -e 'AKIA[0-9A-Z]{16}' -e 'sk-ant-[A-Za-z0-9_-]{20,}' "$tmp/added" |
	cut -f1 | sort -u >"$tmp/shapehits"
while read -r f; do complain "token-shaped string" "$f"; done <"$tmp/shapehits"

# 4: values of credential-looking variables of this very environment, by
# value. Only names are printed. Values shorter than 8 are skipped (noise).
cut -f2- "$tmp/added" >"$tmp/addedtext"
env | sed -n -E "s/^([A-Za-z0-9_]*(TOKEN|SECRET|PASSWORD|KEY|CREDENTIALS?)[A-Za-z0-9_]*)=.*/\1/p" |
	sort -u |
	while read -r n; do
		v=$(printenv "$n")
		[ ${#v} -ge 8 ] || continue
		if grep -Fq -- "$v" "$tmp/addedtext"; then
			echo "$n"
		fi
	done >"$tmp/envhits"
while read -r n; do
	complain "value of environment variable $n appears in the diff" "(see git log -p $range)"
done <"$tmp/envhits"

if [ "$fail" -ne 0 ]; then
	echo "check-push: NOT SAFE TO PUSH. Do not push. Tell the owner." >&2
	exit 1
fi
echo "check-push: ok ($range)"
