complete -c hiddify-tui -f
complete -c hiddify-tui -l address -r -d 'Core gRPC address'
complete -c hiddify-tui -l profile-file -r -d 'Profile store path'
complete -c hiddify-tui -l core-binary -r -d 'Standalone core binary'
complete -c hiddify-tui -l timeout -r -d 'Startup or request timeout'
complete -c hiddify-tui -l json -d 'Produce JSON output'
complete -c hiddify-tui -l no-color -d 'Disable terminal color'
complete -c hiddify-tui -l version -d 'Print version'

for command in status connect disconnect restart profile outbound logs settings migrate install-core
    complete -c hiddify-tui -n "not __fish_seen_subcommand_from status connect disconnect restart profile outbound logs settings migrate install-core" -a "$command"
end
complete -c hiddify-tui -n '__fish_seen_subcommand_from profile' -a 'list show add add-file add-stdin rename activate refresh delete'
complete -c hiddify-tui -n '__fish_seen_subcommand_from outbound' -a 'list select test'
complete -c hiddify-tui -n '__fish_seen_subcommand_from settings' -a 'set validate'
complete -c hiddify-tui -n '__fish_seen_subcommand_from migrate' -a 'gui'
