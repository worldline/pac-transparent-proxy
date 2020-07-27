# A transparent proxy that uses a PAC file to send data to the right proxy

## Usage
```sh
USAGE:
   pac-transparent-proxy [global options] <pac-file-uri>

GLOBAL OPTIONS:
   --debug, -d                            Debug mode (default: false)
   --trace, --dd, --ddd                   Verbose mode (default: false)
   --timeout value, -t value              Connection timeout on TCP connections (default: "30s")
   --pac-file-timeout value, --pft value  Connection timeout on PAC file requests (default: "2s")
   --pac-file-ttl value, --pttl value     TTL on PAC file (default: "60s")
   --port value, -p value                 Listening port (default: 3128)
   --tunnel, -c                           Tunnel HTTP request with HTTP CONNECT (default: true)
   --version, -v                          print the version (default: false)
```

## Install
### Download
```sh
sudo curl -L -o /usr/bin/pac-transparent-proxy https://github.com/worldline/pac-transparent-proxy/releases/latest/download/pac-transparent-proxy
sudo chmod +x /usr/bin/pac-transparent-proxy
```

### Create user for process
```sh
sudo useradd transproxy
```

### Create a systemd service
```sh
sudo bash -c 'cat > /etc/systemd/system/transparent-proxy.service' <<EOL
[Unit]
Description=PAC transparent proxy
After=network.target

[Service]
User=transproxy
PermissionsStartOnly=true

ExecStartPre=-/sbin/iptables -t nat -N TRANSPROXY -w
# Don't intercept connections :
# - when destination in a private network (127.0.0.1/8, 192.168.0.0/16, 172.16.0.0/12, 10.0.0.0/16)
# - from pac-transparent-proxy
ExecStartPre=-/sbin/iptables -t nat -A TRANSPROXY -p tcp -d 127.0.0.1/8,192.168.0.0/16,172.16.0.0/12,10.0.0.0/16 -j RETURN -w
ExecStartPre=-/sbin/iptables -t nat -A TRANSPROXY -p tcp -j REDIRECT --to-ports 12345 -w

# Intercept all TCP trafic
ExecStartPre=-/sbin/iptables -t nat -A OUTPUT  -m owner ! --uid-owner transproxy -p tcp -j TRANSPROXY -w
ExecStartPre=-/sbin/iptables -t nat -A PREROUTING -p tcp -j TRANSPROXY -w

ExecStart=/usr/bin/pac-transparent-proxy -p 12345 http://wpad/wpad.dat

# Clean iptables
ExecStopPost=-/sbin/iptables -t nat -D PREROUTING -p tcp -j TRANSPROXY -w
ExecStopPost=-/sbin/iptables -t nat -D OUTPUT -m owner ! --uid-owner transproxy -p tcp -j TRANSPROXY -w
ExecStopPost=-/sbin/iptables -t nat -F TRANSPROXY -w
ExecStopPost=-/sbin/iptables -t nat -X TRANSPROXY -w

[Install]
WantedBy=multi-user.target
EOL

sudo systemctl daemon-reload
sudo systemctl enable transparent-proxy
sudo systemctl restart transparent-proxy
```
with `http://wpad/wpad.dat` the URI of your PAC file.
