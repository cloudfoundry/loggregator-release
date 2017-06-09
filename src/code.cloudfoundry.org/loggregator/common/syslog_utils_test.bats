#!/usr/bin/env bats

setup() {
  source ./syslog_utils.sh

  TMPDIR=$(mktemp -dt "syslog_util")
}

@test "tee_output_to_sys_log should create the correct logfile" {
  run tee_output_to_sys_log ${TMPDIR}
  [ "$status" -eq 0 ]
  [ -e "${TMPDIR}/bats-exec-test.log" ]
  [ -e "${TMPDIR}/bats-exec-test.err.log" ]
}

@test "tee_output_to_sys_log requires non empty log_dir" {
  run tee_output_to_sys_log
  [ "$status" -eq 1 ]
}

@test "tee_output_to_sys_log requires valid log_dir" {
  run tee_output_to_sys_log /invalid/directory
  [ "$status" -eq 2 ]
}
