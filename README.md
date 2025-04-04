# Energy Tracker Module

Track energy usage from another power sensor.

## Module

Keeps track of energy usage from a power sensor. Proxies the underlying sensors instantaneous voltage, current, and power readings while keeping tracking energy and current usage over time. Current tracking is only useful for constant current applications or tracking battery capacity.

### Configuration

A underlying Power Sensor must already be configured.
The refresh frequency should be the lowest value that is supported by the `source sensor`. Higher values will result in less frequent readings and higher error in results.

#### Attributes

The following attributes are available for this model:

| Name          | Type   | Inclusion | Description                |
|---------------|--------|-----------|----------------------------|
| `source_sensor` | string  | Required  | The name of the Power Sensor to get readings |
| `refresh_rate_msec` | int | Optional  | Refresh frequency of data from the `source_sensor`. Default: 2000 |

#### Example Configuration

```json
{
  "source_sensor": "power-1",
  "refresh_rate_msec": 5000
}
```

### Output

GetVoltage(), GetCurrent(), and GetPower() are proxied to the `source_sensor`.
GetReadings() gets two additional values -- "total_energy_Wh" and "total_current_Ah", both as floats.

### DoCommand

To reset "total_energy_Wh" and "total_current_Ah" to zero, send the following DoCommand:
```json
{
  "reset" : true
}
```

