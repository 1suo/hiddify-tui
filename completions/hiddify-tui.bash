_hiddify_tui() {
    local current previous commands
    COMPREPLY=()
    current="${COMP_WORDS[COMP_CWORD]}"
    previous="${COMP_WORDS[COMP_CWORD-1]}"
    commands="status connect disconnect restart profile outbound logs settings migrate install-core"

    case "$previous" in
        --address|--profile-file|--core-binary|--timeout|--profile|--name|--database|--configs|--level)
            return
            ;;
        hiddify-tui)
            COMPREPLY=( $(compgen -W "$commands --json --no-color --version --address --profile-file --core-binary --timeout" -- "$current") )
            return
            ;;
        profile)
            COMPREPLY=( $(compgen -W "list show add add-file add-stdin rename activate refresh delete" -- "$current") )
            return
            ;;
        outbound)
            COMPREPLY=( $(compgen -W "list select test" -- "$current") )
            return
            ;;
        settings)
            COMPREPLY=( $(compgen -W "set validate" -- "$current") )
            return
            ;;
        migrate)
            COMPREPLY=( $(compgen -W "gui" -- "$current") )
            return
            ;;
    esac
}
complete -F _hiddify_tui hiddify-tui
