
# tee_output_to_sys_log
#
# When syslog_utils.sh is loaded, this sends stdout and stderr to /var/vcap/sys/log.
function tee_output_to_sys_log() {
  declare log_dir="$1"

  local log_basename
  log_basename="$(basename "$0")"

  exec > >(tee -a >(logger -p user.info -t "vcap.${log_basename}.stdout") | prepend_datetime >>"${log_dir}/${log_basename}.log")
  exec 2> >(tee -a >(logger -p user.error -t "vcap.${log_basename}.stderr") | prepend_datetime >>"${log_dir}/${log_basename}.err.log")
}

function prepend_datetime() {
  awk -W interactive '{lineWithDate="echo [`date +\"%Y-%m-%d %H:%M:%S%z\"`] \"" $0 "\""; system(lineWithDate)  }'
}
