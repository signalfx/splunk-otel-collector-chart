import os
import time
import logging
import sys

logger = logging.getLogger(__name__)
logger.setLevel(logging.INFO)
formatter = logging.Formatter("%(message)s")
handler = logging.StreamHandler(sys.stdout)
handler.setFormatter(formatter)
logger.addHandler(handler)


DEFAULT_LOGS_DIR = "logs/"
GET_PODS_FILE_NAME = DEFAULT_LOGS_DIR + "get_pods.out"
AGENT_POD_LOGS = DEFAULT_LOGS_DIR + "agent_pod_logs.out"


def get_pod_full_name(pod):
    create_dir_if_not_exists(DEFAULT_LOGS_DIR)
    os.system("kubectl get pods > " + GET_PODS_FILE_NAME)
    lines = get_log_file_content(GET_PODS_FILE_NAME)
    for line in lines:
        tmp = line.split()
        logger.info(tmp)
        if pod in tmp[0]:
            logger.info(f"{pod} full name is: {tmp[0]}")
            return tmp[0]
    return "pod_name_not_found"


def get_log_file_content(log_file_name):
    with open(log_file_name) as f:
        lines = f.readlines()
    f.close()
    return lines


def get_pod_logs(pod_full_name):
    create_dir_if_not_exists(DEFAULT_LOGS_DIR)
    os.system(f"kubectl logs {pod_full_name} > {AGENT_POD_LOGS}")
    return get_log_file_content(AGENT_POD_LOGS)


def check_if_upgrade_successful(upgrade_log_name):
    upgrade_success_log = "has been upgraded. Happy Helming!"
    lines = get_log_file_content(upgrade_log_name)
    for line in lines:
        if upgrade_success_log in line:
            logger.info("upgrade successful")
            return True
    logger.error("upgrade failed")
    logger.info(lines)
    return False


def prepare_set_yaml_fields_cmd(fields_dict):
    cmd = ""
    if fields_dict != None:
        for k, v in fields_dict.items():
            cmd = cmd + " --set " + k + "=" + v
    return cmd


def create_dir_if_not_exists(dir_name):
    # Check whether the specified path exists or not
    is_exist = os.path.exists(dir_name)
    if not is_exist:
        os.makedirs(dir_name)
        logger.info("The new directory is created!")


def upgrade_helm(yaml_file, fields_dict=None):
    logger.info("=======================")
    create_dir_if_not_exists(DEFAULT_LOGS_DIR)
    upgrade_sck_log = DEFAULT_LOGS_DIR + "upgrade.log"
    set_yaml_fields_cmd = prepare_set_yaml_fields_cmd(fields_dict)
    os.system(
        f"helm upgrade ci-sck --values {yaml_file}"
        + set_yaml_fields_cmd
        + f" ./../helm-charts/splunk-otel-collector/ > {upgrade_sck_log}"
    )
    check_if_upgrade_successful(upgrade_sck_log)
    wait_for_pods_initialization()


def wait_for_pods_initialization():
    create_dir_if_not_exists(DEFAULT_LOGS_DIR)
    break_infinite_looping_counter = 60
    for x in range(break_infinite_looping_counter):
        time.sleep(1)
        counter = 0
        get_pods_logs = DEFAULT_LOGS_DIR + "get_pods_wait_for_pods.log"
        os.system(f"kubectl get pods > {get_pods_logs}")
        lines = get_log_file_content(get_pods_logs)
        # skip first line/row - header row
        for line in lines[1:]:
            if "Running" == line.split()[2]:
                counter += 1
            else:
                logger.info(
                    f"Not ready pod: {line.split()[0]}, status: {line.split()[2]}"
                )
        if counter == len(lines) - 1:
            break
    time.sleep(5)  # wait for ingesting logs into splunk after connector is ready
