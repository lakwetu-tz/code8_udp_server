# Create a service file at

    touch /etc/systemd/system/my-go-daemon.service

Enter

    [Unit]
    Description=My Go App
    
    [Service]
    Type=simple
    WorkingDirectory=/my/go/app/directory
    ExecStart=/usr/lib/go run main.go 
    
    [Install]
    WantedBy=multi-user.target
  
Then enable and start the service

    systemctl enable my-go-daemon
    systemctl start my-go-daemon
    systemctl status my-go-daemon
  
systemd has a separate journaling system that will let you tail logs for easy trouble-shooting.
