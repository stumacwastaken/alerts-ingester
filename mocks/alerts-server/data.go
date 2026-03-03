package main

import "math/rand/v2"

type pickableList []string

func (s pickableList) pickOne() string {
	selection := rand.IntN(len(s) - 1)
	return s[selection]
}

var sourceList pickableList
var severityTypeList pickableList
var alertDescriptionList pickableList

func initDataSources() {
	sourceList = pickableList{
		"siem-1",
		"siem-2",
		"siem-3",
		"siem-4",
		"siem-5",
		"siem-6",
		"siem-7",
		"siem-8",
		"siem-9",
		"siem-10",
	}

	// severity - may be one of: low, medium, high, critical
	severityTypeList = pickableList{
		"low",
		"medium",
		"high",
		"critical",
	}

	// I asked AI to generate a list of random descriptions. They have no
	// bearing on the severity. Could have been done with faker-like library and
	// lorem ipsum sentences, but I felt like avoiding any deps for the mock.
	alertDescriptionList = pickableList{
		"Brute force attempt detected on admin account",
		"Multiple failed logins from isolated internal IP",
		"Unauthorized PowerShell execution detected",
		"Lateral movement: SMB session to domain controller",
		"Large data egress to unauthorized cloud storage",
		"SQL Injection pattern detected on web server",
		"Suspicious scheduled task created by SYSTEM",
		"DDoS signature detected on edge firewall",
		"Anomalous VPN login from geolocated blacklisted IP",
		"Privilege escalation: User added to Domain Admin",
		"Ransomware signature: Rapid file encryption",
		"DNS tunneling activity detected on workstation",
		"Mimikatz execution detected in memory",
		"Inbound traffic from known Tor exit node",
		"Outbound connection to suspected C2 server",
		"Registry modification: Persistence mechanism added",
		"Potential Golden Ticket attack detected",
		"Clearing of Windows Security Event Logs",
		"Impossible travel: Login from distant locations",
		"New administrative share created on workstation",
		"Unusual peak in CPU usage on database server",
		"Unauthorized access to sensitive HR folder",
		"SSH brute force detected on Linux jumpbox",
		"Process hollowing detected in svchost.exe",
		"Excessive account lockouts in short duration",
		"Weak TLS version used in external communication",
		"Malicious macro detected in email attachment",
		"Active Directory replication from non-DC source",
		"Web shell uploaded to public-facing server",
		"Credential harvesting page accessed by user",
		"Unauthorized USB device plugged into endpoint",
		"Bypass of Multi-Factor Authentication (MFA)",
		"Service account login from non-standard host",
		"Kerberoasting activity: SPN ticket request",
		"Port scanning activity from internal subnet",
		"Local admin group membership changed",
		"EICAR test file detected by antivirus",
		"Modification of system hosts file",
		"Insecure direct object reference (IDOR) attempt",
		"Cross-Site Scripting (XSS) payload detected",
		"Internal IP accessing dark web resources",
		"Unsigned driver loaded into kernel space",
		"WMI persistence script executed",
		"Pass-the-Hash attempt detected",
		"Sensitive file downloaded from SharePoint",
		"Firewall rule modified to allow all traffic",
		"NMAP scan detected against internal assets",
		"Beaconing behavior to known malicious domain",
		"Unexpected shutdown of security agent",
		"Large volume of emails sent to external domain",
		"Rootkit signature detected on production server",
		"Suspicious API call to AWS Metadata service",
		"Unauthorized S3 bucket policy change",
		"Cryptomining traffic detected on port 4444",
		"LSASS memory dump attempted",
		"Unauthorized change to DNS settings",
		"Access to sensitive data by terminated user",
		"Domain-level GPO modification detected",
		"Suspicious RDP session duration",
		"Inbound RDP from public internet",
		"Shadow IT: Unauthorized SaaS application used",
		"Password spraying attack in progress",
		"Remote file inclusion (RFI) attempt",
		"Local File Inclusion (LFI) attempt",
		"HTTP 500 errors spike on checkout page",
		"Unauthorized DHCP server detected",
		"ARP spoofing attempt on local segment",
		"Honeytoken account accessed",
		"Access to critical infra via legacy protocol",
		"Unusual volume of print jobs to external IP",
		"Unauthorized VPN tunnel established",
		"Credential stuffing against login portal",
		"Base64 encoded string in command line",
		"Discord/Slack used for data exfiltration",
		"GitHub token found in public repository",
		"Excessive DNS queries for non-existent domains",
		"Privileged access without ticket reference",
		"Container escape attempt detected",
		"Kubernetes API unauthorized access",
		"Docker socket mounted in privileged container",
		"Sensitive ENV variables accessed in CI/CD",
		"Unauthorized VPC peering connection",
		"Route53 record modified unexpectedly",
		"IAM policy weakened for root user",
		"Azure Global Admin role assigned",
		"GCP service account key created",
		"Suspicious login to Snowflake warehouse",
		"Zero-day exploit pattern detected",
		"Buffer overflow attempt on legacy app",
		"Directory traversal attack detected",
		"Unauthorized modification of /etc/shadow",
		"New sudoers entry added by non-root",
		"Java deserialization exploit attempt",
		"Log4j JNDI lookup signature detected",
		"Web traffic to IP with no reverse DNS",
		"Abnormal database query size",
		"Suspicious change in backup schedule",
		"Data restoration from unknown source",
		"Unauthorized mirror port configuration",
		"Wi-Fi Pineapple detected in office",
	}
}
