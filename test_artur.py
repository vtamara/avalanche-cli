def test_passwd_file(host):
    passwd = host.file("/etc/passwd")
    assert passwd.contains("root")
    assert passwd.user == "root"
    assert passwd.group == "root"
    assert passwd.mode == 0o644


def test_curl_is_installed(host):
    curl = host.package("curl")
    assert curl.is_installed
    assert curl.version.startswith("7.68")


def test_avalanchego_running_and_enabled(host):
    avalanchego = host.service("avalanchego")
    assert avalanchego.is_running
    assert avalanchego.is_enabled
