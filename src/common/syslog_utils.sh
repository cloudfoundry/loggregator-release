
# tee_output_to_sys_log
#
# When syslog_utils.sh is loaded, this sends stdout and stderr to /var/vcap/sys/log.
#
# Params:
#  - $1 logdir: directory to write the log files
#  - $2 log_basename: optional, basename for the syslog tag and log file
#  - $3 stdout_suffix: suffix for stdout log file
#  - $4 stderr_suffix: suffix for stderr log file
function tee_output_to_sys_log() {
  declare log_dir="$1"
  shift
  if [ "$log_dir" = "" ] ; then
    return 1
  fi

  if [ ! -e "$log_dir" ] ; then
    return 2
  fi

  declare log_basename="${1:-$(basename "$0")}"
  shift

  declare stdout_suffix="${1:-}"
  stdout_suffix="${stdout_suffix:+.${stdout_suffix}}"
  shift

  declare stderr_suffix="${1:-err}"
  stderr_suffix="${stderr_suffix:+.${stderr_suffix}}"
  shift

  exec > >(tee -a >(logger -p user.info -t "vcap.${log_basename}.stdout") | prepend_datetime >>"${log_dir}/${log_basename}${stdout_suffix}.log")
  exec 2> >(tee -a >(logger -p user.error -t "vcap.${log_basename}.stderr") | prepend_datetime >>"${log_dir}/${log_basename}${stderr_suffix}.log")
}

function prepend_datetime() {
  awk -W interactive '{lineWithDate="echo [`date +\"%Y-%m-%d %H:%M:%S%z\"`] \"" $0 "\""; system(lineWithDate)  }'
}
