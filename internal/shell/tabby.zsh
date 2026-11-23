#!/usr/bin/env zsh
# shellcheck disable=SC2296
# shellcheck disable=SC2034

typeset -ga _tabby_matches
typeset -gi _tabby_index=0
typeset -g _tabby_fallback_widget
typeset -g _tabby_pending_command
typeset -g _tabby_pending_dir
typeset -g _tabby_query
typeset -g _tabby_pending_query
typeset -g _tabby_previous

_tabby_reset() {
	_tabby_matches=()
	_tabby_index=0
	_tabby_query=
}

_tabby_fallback() {
	local widget="${1:-$_tabby_fallback_widget}"
	_tabby_reset
	zle "$widget"
}

_tabby_cycle() {
	local direction="$1"
	local fallback="$2"
	local output

	if (( ${#_tabby_matches} > 0 )) &&
		[[ "$BUFFER" == "${_tabby_matches[$((_tabby_index + 1))]}" ]] &&
		(( CURSOR == ${#BUFFER} )); then
		_tabby_index=$(( (_tabby_index + direction + ${#_tabby_matches}) % ${#_tabby_matches} ))
		BUFFER="${_tabby_matches[$((_tabby_index + 1))]}"
		CURSOR=${#BUFFER}
		POSTDISPLAY=
		return
	fi

	_tabby_reset

	if (( CURSOR != ${#BUFFER} )); then
		_tabby_fallback "$fallback"
		return
	fi

	if [[ -z "$BUFFER" ]]; then
		[[ -n "$_tabby_previous" ]] &&
			output="$(command tabby next -- "$_tabby_previous" 2>/dev/null)"
	else
		output="$(command tabby query -- "$BUFFER" 2>/dev/null)" || output=
	fi
	if [[ -z "$output" ]]; then
		_tabby_fallback "$fallback"
		return
	fi
	_tabby_query="$BUFFER"
	_tabby_matches=("${(@f)output}")
	(( direction < 0 )) && _tabby_index=$((${#_tabby_matches} - 1))

	BUFFER="${_tabby_matches[$((_tabby_index + 1))]}"
	CURSOR=${#BUFFER}
	POSTDISPLAY=
}

_tabby_complete() {
	_tabby_cycle 1 "$_tabby_fallback_widget"
}

_tabby_complete_reverse() {
	_tabby_cycle -1 reverse-menu-complete
}

_tabby_record_preexec() {
	_tabby_pending_command="$1"
	_tabby_pending_dir="$PWD"
	_tabby_pending_query=
	if (( ${#_tabby_matches} > 0 )) &&
		[[ "$1" == "${_tabby_matches[$((_tabby_index + 1))]}" ]]; then
		_tabby_pending_query="$_tabby_query"
	fi
	_tabby_reset
}

_tabby_record_precmd() {
	local exit_status=$?
	local executed="$_tabby_pending_command"
	local dir="$_tabby_pending_dir"
	local query="$_tabby_pending_query"
	_tabby_pending_command=
	_tabby_pending_dir=
	_tabby_pending_query=
	_tabby_reset

	if (( exit_status != 0 )) || [[ $executed == [[:space:]]* ]]; then
		_tabby_previous=
		return 0
	fi
	[[ -n $executed ]] || return 0

	TABBY_DIR="$dir" command tabby add --query="$query" --previous="$_tabby_previous" -- "$executed" >/dev/null 2>&1 &!
	_tabby_previous="$executed"
	return 0
}

if [[ -z "$_tabby_fallback_widget" ]]; then
	typeset -a _tabby_binding
	_tabby_binding=("${(z)$(bindkey '^I')}")
	_tabby_fallback_widget="${_tabby_binding[2]:-expand-or-complete}"
	unset _tabby_binding
fi

export TABBY_SHELL=zsh

zle -N _tabby_complete
zle -N _tabby_complete_reverse
bindkey '^I' _tabby_complete
bindkey '^[[Z' _tabby_complete_reverse

autoload -Uz add-zsh-hook
add-zsh-hook -d preexec _tabby_record_preexec 2>/dev/null
add-zsh-hook -d precmd _tabby_record_precmd 2>/dev/null
add-zsh-hook preexec _tabby_record_preexec
add-zsh-hook precmd _tabby_record_precmd
