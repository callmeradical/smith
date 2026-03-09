#!/usr/bin/env bash
set -euo pipefail

IMAGE="${1:-loop-base:local}"
SKILLS_PATH="/home/dev/.codex/skills"

host_skills_dir="$(mktemp -d)"
trap 'rm -rf "$host_skills_dir"' EXIT

sentinel_file="host-skill.txt"
sentinel_content="skills-mount-ok"
printf '%s\n' "$sentinel_content" > "${host_skills_dir}/${sentinel_file}"

docker run --rm \
  -v "${host_skills_dir}:${SKILLS_PATH}" \
  "$IMAGE" sh -lc "
    set -eu
    test -d '${SKILLS_PATH}'
    test -r '${SKILLS_PATH}'
    test -f '${SKILLS_PATH}/${sentinel_file}'
    value=\$(cat '${SKILLS_PATH}/${sentinel_file}')
    test \"\$value\" = '${sentinel_content}'
  "

host_value="$(cat "${host_skills_dir}/${sentinel_file}")"
if [[ "$host_value" != "$sentinel_content" ]]; then
  echo "mounted skills content changed unexpectedly" >&2
  exit 1
fi

docker run --rm "$IMAGE" sh -lc "
  set -eu
  test -d '${SKILLS_PATH}'
  test -r '${SKILLS_PATH}'
  [ -z \"\$(ls -A '${SKILLS_PATH}')\" ]
"

echo "skills mount smoke check passed"
