#!/usr/bin/env bash

[[ $- == *i* ]] || return

if (( BASH_VERSINFO[0] < 4 || BASH_VERSINFO[0] == 4 && BASH_VERSINFO[1] < 4 )); then
	printf 'tabby: Bash 4.4 or newer is required\n' >&2
	return
fi

_tabby_query=
_tabby_query_matches=()
_tabby_previous=

_tabby_complete() {
	local match
	local -a request
	COMPREPLY=()

	(( COMP_POINT == ${#COMP_LINE} )) || return 0

	if [[ -z ${COMP_LINE//[[:space:]]/} ]]; then
		[[ -n $_tabby_previous ]] || return 0
		request=(next -- "$_tabby_previous")
	else
		request=(query -- "$COMP_LINE")
	fi

	while IFS= read -r match; do
		[[ -n $match ]] && COMPREPLY+=("$match")
	done < <(command tabby "${request[@]}" 2>/dev/null)

	if (( ${#COMPREPLY[@]} > 0 )); then
		_tabby_query=$COMP_LINE
		_tabby_query_matches=("${COMPREPLY[@]}")
		compopt -o noquote -o nosort -o nospace
	fi
}

_tabby_record_prompt() {
	local exit_status=$?
	local entry history_id executed match query=
	local HISTTIMEFORMAT=
	local last_query=$_tabby_query
	local -a last_matches=("${_tabby_query_matches[@]}")
	_tabby_query=
	_tabby_query_matches=()

	entry="$(builtin history 1)"
	if [[ ! $entry =~ ^[[:space:]]*([0-9]+)[[:space:]]+(.*)$ ]]; then
		return 0
	fi

	history_id=${BASH_REMATCH[1]}
	executed=${BASH_REMATCH[2]}

	if [[ -z ${_tabby_history_ready:-} ]]; then
		_tabby_history_ready=1
		_tabby_last_history_id=$history_id
		return 0
	fi

	[[ $history_id != "${_tabby_last_history_id:-}" ]] || return 0
	_tabby_last_history_id=$history_id

	if (( exit_status != 0 )); then
		_tabby_previous=
		return 0
	fi

	for match in "${last_matches[@]}"; do
		if [[ $match == "$executed" ]]; then
			query=$last_query
			break
		fi
	done

	( command tabby add --query="$query" --previous="$_tabby_previous" -- "$executed" >/dev/null 2>&1 & )
	_tabby_previous=$executed
	return 0
}

complete -I -F _tabby_complete -o bashdefault -o default
complete -E -F _tabby_complete -o bashdefault -o default
bind '"\C-i": menu-complete'
bind '"\e[Z": menu-complete-backward'

if [[ ${PROMPT_COMMAND:-} != *'_tabby_record_prompt'* ]]; then
	PROMPT_COMMAND="_tabby_record_prompt${PROMPT_COMMAND:+; $PROMPT_COMMAND}"
fi
