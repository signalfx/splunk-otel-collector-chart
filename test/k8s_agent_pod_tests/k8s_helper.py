import os
import time

GET_PODS_FILE_NAME = "../k8s_correctness_tests/get_pods.out"
AGENT_POD_LOGS = "agent_pod_logs.out"


def get_pod_full_name(pod):
    os.system("kubectl get pods > " + GET_PODS_FILE_NAME)
    lines = get_log_file_content(GET_PODS_FILE_NAME)
    for line in lines:
        tmp = line.split()
        print(tmp)
        if pod in tmp[0]:
            print(f"{pod} full name is: {tmp[0]}")
            return tmp[0]
    return "pod_name_not_found"


def get_log_file_content(log_file_name):
    with open(log_file_name) as f:
        lines = f.readlines()
    f.close()
    return lines


def get_pod_logs(pod_full_name):
    os.system(f"kubectl logs {pod_full_name} > {AGENT_POD_LOGS}")
    return get_log_file_content(AGENT_POD_LOGS)


def check_if_upgrade_successful(upgrade_log_name):
    upgrade_success_log = "has been upgraded. Happy Helming!"
    lines = get_log_file_content(upgrade_log_name)
    for line in lines:
        if upgrade_success_log in line:
            print("upgrade successful")
            return True
    print("upgrade failed")
    print(lines)
    return False


def upgrade_helm(yaml_file):
    print("=======================")
    upgrade_sck_log = "upgrade.log"

    # print(yaml_file)
    # os.system(f"cat {yaml_file}")
    os.system("pwd")
    print("=====================================================================")
    # os.system(f"helm install ci-sck --values {yaml_file} ./../../splunk-otel-collector-chart/splunk-otel-collector")
    # os.system(f"helm install ci-sck --values {yaml_file} ./../../helm-charts/splunk-otel-collector/")
    host = os.environ.get("CI_SPLUNK_HOST")
    token = os.environ.get("CI_SPLUNK_HEC_TOKEN")
    os.system('echo $CI_SPLUNK_HEC_TOKEN')
    print(token)
    # print(f"host {host}")
    # print(f"helm upgrade ci-sck --values {yaml_file} --set splunkPlatform.endpoint=https://{host}:8088/services/collector \ ./../../helm-charts/splunk-otel-collector/ > {upgrade_sck_log}")
    os.system(
        # f"helm upgrade ci-sck --values {yaml_file} --set splunkPlatform.endpoint=https://{host}:8088/services/collector \ ./../helm-charts/splunk-otel-collector/ > {upgrade_sck_log}"
        f"helm upgrade ci-sck --values {yaml_file} --set splunkPlatform.endpoint=https://{host}:8088/services/collector \ ./../../helm-charts/splunk-otel-collector/ > {upgrade_sck_log}"
    )
    check_if_upgrade_successful(upgrade_sck_log)
    os.system("env | grep CI_")
    print("=====================================================================")
    # time.sleep(10)
    wait_for_pods_initialization()


def wait_for_pods_initialization():
    #   script_body = f"""
    # until kubectl get pod | grep Running | [[ $(wc -l) == 1 ]]; do
    #   sleep 1;
    # done"""
    #   with open("check_for_pods.sh", "w") as fp:
    #       fp.write(script_body)
    #   os.system("chmod a+x check_for_pods.sh && ./check_for_pods.sh")
    break_infinite_looping_counter = 60
    for x in range(break_infinite_looping_counter):
        time.sleep(1)
        counter = 0
        get_pods_logs = "get_pods_wait_for_pods.log"
        os.system(f"kubectl get pods > {get_pods_logs}")
        lines = get_log_file_content(get_pods_logs)
        for line in lines:
            if "Running" == line.split()[2]:
                counter += 1
            else:
                print(f"Not ready pod: {line.split()[0]}, status: {line.split()[2]}")

        if counter == len(lines) - 1:
            break


if __name__ == "__main__":
    print("start")
    agent_pod = get_pod_full_name("agent")
    print(agent_pod)
    logs = get_pod_logs(agent_pod)
    print(len(logs))
    print("stop")
