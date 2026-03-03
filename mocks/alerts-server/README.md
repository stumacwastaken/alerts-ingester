# Alerts API
This is a mock api service that the primary "alerts ingester" service uses to
fetch data. It generates random data per request and passes it along in the
format defined by the take home specs.

Don't expect anything crazy here. This is a quick and dirty mock.


## From the takehome: 
Third-party Alerts API (simulated) 

Assume the upstream “Alerts API” exposes: 

GET /alerts?since=<ISO8601>

Example:
```json
{

  "alerts": [

    {

      "source": "siem-1",

      "severity": "high",

      "description": "Suspicious login",

      "created_at": "2025-01-10T12:34:56Z"

    }

  ]

}
```

severity - may be one of: low, medium, high, critical

source - should be limited to 10 sources of your choice 