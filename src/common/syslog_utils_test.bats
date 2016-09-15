#!/usr/bin/env bats

setup() {
  source ./syslog_utils.sh

  TMPDIR=$(mktemp -dt "syslog_util_XXX")
}

@test "tee_output_to_sys_log should create the correct logfile when not defining log_basename" {
  run tee_output_to_sys_log ${TMPDIR}
  [ "$status" -eq 0 ]
  [ -e "${TMPDIR}/bats-exec-test.log" ]
  [ -e "${TMPDIR}/bats-exec-test.err.log" ]
}

@test "tee_output_to_sys_log should create the correct logfile when defining log_basename" {
  run tee_output_to_sys_log ${TMPDIR} "foo"
  ls $TMPDIR
  [ "$status" -eq 0 ]
  [ -e "${TMPDIR}/foo.log" ]
  [ -e "${TMPDIR}/foo.err.log" ]
}

@test "tee_output_to_sys_log should create the correct logfile when defining log_basename and stdout/stderr suffix" {
  run tee_output_to_sys_log ${TMPDIR} "foo" "stdout" "stderr"
  [ "$status" -eq 0 ]
  [ -e "${TMPDIR}/foo.stdout.log" ]
  [ -e "${TMPDIR}/foo.stderr.log" ]
}

@test "tee_output_to_sys_log requires non empty log_dir" {
  run tee_output_to_sys_log
  [ "$status" -eq 1 ]
}

@test "tee_output_to_sys_log requires valid log_dir" {
  run tee_output_to_sys_log /invalid/directory
  [ "$status" -eq 2 ]
}
