# WZKY

Small reverse proxy to help old Windows Servers with limited crypto support communicate securely.

## Running as a service on Windows

Store executable in: `C:\Program Files\wzky\wzky_windows_amd64.exe`

```cmd
SC CREATE wzky-httpbin-proxy binPath= "C:\Program Files\wzky\wzky_windows_amd64.exe --service-name wzky-httpbin-proxy --host httpbin.org --port 1337" DisplayName= "wzky Httpbin Proxy" start= auto
```

Remove the service:

```cmd
SC DELETE wzky-httpbin-proxy
```

# License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
