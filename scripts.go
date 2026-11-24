package main

// Every PowerShell script Astro Tweaks runs is built here, in platform-neutral code,
// so scripts_test.go can syntax-check all of them with the real PowerShell parser.

import (
	"encoding/json"
	"fmt"
	"strings"
)

const psPrefix = "$ProgressPreference='SilentlyContinue';$ErrorActionPreference='Stop';[Console]::OutputEncoding=[System.Text.Encoding]::UTF8;"

func psQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

type taskRec struct {
	P string `json:"p"`
	N string `json:"n"`
	S string `json:"s"`
	E string `json:"e,omitempty"`
}

type nicRec struct {
	N string `json:"n"`
	K string `json:"k"`
	V string `json:"v"`
}

type nicBackup struct {
	Props []nicRec `json:"props"`
	PM    []string `json:"pm"`
}

func psApplyTasks(paths []string) string {
	var q []string
	for _, p := range paths {
		q = append(q, psQuote(p))
	}
	return `$names=@(` + strings.Join(q, ",") + `); $r=New-Object System.Collections.ArrayList
foreach($t in $names){
  $i=$t.LastIndexOf('\')
  if($i -ge 0){ $x=@(Get-ScheduledTask -TaskPath $t.Substring(0,$i+1) -TaskName $t.Substring($i+1) -ErrorAction SilentlyContinue) } else { $x=@(Get-ScheduledTask -TaskName $t -ErrorAction SilentlyContinue) }
  foreach($k in $x){
    $o=[ordered]@{p=$k.TaskPath;n=$k.TaskName;s=[string]$k.State}
    if($o.s -ne 'Disabled'){ try{ Disable-ScheduledTask -TaskPath $k.TaskPath -TaskName $k.TaskName -ErrorAction Stop | Out-Null }catch{ $o.e=$_.Exception.Message } }
    [void]$r.Add([pscustomobject]$o)
  }
}
ConvertTo-Json -Compress -InputObject @($r)`
}

func psRevertTasks(recs []taskRec) string {
	var lines []string
	for _, r := range recs {
		lines = append(lines, fmt.Sprintf("try{Enable-ScheduledTask -TaskPath %s -TaskName %s -ErrorAction Stop|Out-Null}catch{}", psQuote(r.P), psQuote(r.N)))
	}
	return strings.Join(lines, "\n")
}

func psBitLocker() string {
	return `$v=Get-BitLockerVolume -MountPoint $env:SystemDrive -ErrorAction Stop; if($v.ProtectionStatus -eq 'On'){ Suspend-BitLocker -MountPoint $env:SystemDrive -RebootCount 1 -ErrorAction Stop | Out-Null; 'suspended' } else { 'off' }`
}

func psNic(pairs [][2]string, powerMgmt bool) string {
	var kv []string
	for _, p := range pairs {
		kv = append(kv, "@("+psQuote(p[0])+","+psQuote(p[1])+")")
	}
	// Keywords like *EEE start with a literal asterisk, so match them exactly instead of as wildcards.
	s := `$pairs=@(` + strings.Join(kv, ",") + `); $props=New-Object System.Collections.ArrayList; $pm=New-Object System.Collections.ArrayList
$ads=@(Get-NetAdapter -Physical -ErrorAction SilentlyContinue | Where-Object { $_.Status -ne 'Disabled' })
foreach($a in $ads){
  $all=@(Get-NetAdapterAdvancedProperty -Name $a.Name -AllProperties -ErrorAction SilentlyContinue)
  foreach($p in $pairs){
    $cur=$all | Where-Object { $_.RegistryKeyword -ceq $p[0] } | Select-Object -First 1
    if($cur -and $cur.RegistryValue -and ([string]$cur.RegistryValue[0]) -ne $p[1]){
      try{ Set-NetAdapterAdvancedProperty -InputObject $cur -RegistryValue $p[1] -NoRestart -ErrorAction Stop; [void]$props.Add([pscustomobject]@{n=$a.Name;k=$p[0];v=[string]$cur.RegistryValue[0]}) }catch{}
    }
  }`
	if powerMgmt {
		s += `
  try{ $m=Get-NetAdapterPowerManagement -Name $a.Name -ErrorAction Stop; if([string]$m.AllowComputerToTurnOffDevice -eq 'Enabled'){ Set-NetAdapterPowerManagement -Name $a.Name -AllowComputerToTurnOffDevice Disabled -NoRestart -ErrorAction Stop; [void]$pm.Add($a.Name) } }catch{}`
	}
	s += `
}
if($ads.Count -eq 0){ 'NOADAPTER' } else { ConvertTo-Json -Compress -Depth 3 -InputObject ([pscustomobject]@{props=@($props);pm=@($pm)}) }`
	return s
}

func psRevertNic(bk nicBackup) string {
	var lines []string
	for _, p := range bk.Props {
		lines = append(lines, fmt.Sprintf("try{$c=@(Get-NetAdapterAdvancedProperty -Name %s -AllProperties -ErrorAction Stop) | Where-Object { $_.RegistryKeyword -ceq %s } | Select-Object -First 1; if($c){ Set-NetAdapterAdvancedProperty -InputObject $c -RegistryValue %s -NoRestart -ErrorAction Stop }}catch{}", psQuote(p.N), psQuote(p.K), psQuote(p.V)))
	}
	for _, n := range bk.PM {
		lines = append(lines, fmt.Sprintf("try{Set-NetAdapterPowerManagement -Name %s -AllowComputerToTurnOffDevice Enabled -NoRestart -ErrorAction Stop}catch{}", psQuote(n)))
	}
	return strings.Join(lines, "\n")
}

func psUSBApply() string {
	return `$r=New-Object System.Collections.ArrayList
Get-CimInstance -Namespace root\wmi -ClassName MSPower_DeviceEnable -ErrorAction Stop | Where-Object { $_.Enable -and ($_.InstanceName -match '^(USB|HID)\\') } | ForEach-Object { try { Set-CimInstance -InputObject $_ -Property @{Enable=$false} -ErrorAction Stop; [void]$r.Add($_.InstanceName) } catch {} }
ConvertTo-Json -Compress -InputObject @($r)`
}

func psUSBRevert(names []string) string {
	j, _ := json.Marshal(names)
	return `$names=@(ConvertFrom-Json ` + psQuote(string(j)) + `)
Get-CimInstance -Namespace root\wmi -ClassName MSPower_DeviceEnable -ErrorAction Stop | Where-Object { $names -contains $_.InstanceName } | ForEach-Object { try { Set-CimInstance -InputObject $_ -Property @{Enable=$true} -ErrorAction Stop } catch {} }`
}

func psMMApply(feat string) string {
	return `$m=Get-MMAgent; if($m.` + feat + `){ Disable-MMAgent -` + feat + `; 'WAS_ON' } else { 'WAS_OFF' }`
}
func psMMRevert(feat string) string { return `Enable-MMAgent -` + feat }

func psDNSGet() string {
	return `(Get-NetAdapter -Physical | Where-Object Status -eq 'Up' | ForEach-Object { (Get-DnsClientServerAddress -InterfaceIndex $_.ifIndex -AddressFamily IPv4).ServerAddresses } | Select-Object -Unique) -join ','`
}

func psDNSSet(ips []string) string {
	if len(ips) == 0 {
		return `Get-NetAdapter -Physical | Where-Object Status -eq 'Up' | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex -ResetServerAddresses }; Clear-DnsClientCache`
	}
	var q []string
	for _, ip := range ips {
		q = append(q, psQuote(ip))
	}
	return `Get-NetAdapter -Physical | Where-Object Status -eq 'Up' | ForEach-Object { Set-DnsClientServerAddress -InterfaceIndex $_.ifIndex -ServerAddresses @(` + strings.Join(q, ",") + `) }; Clear-DnsClientCache`
}

func psRestorePoint(label string) string {
	return `$k='HKLM:\SOFTWARE\Microsoft\Windows NT\CurrentVersion\SystemRestore'
$old=(Get-ItemProperty -Path $k -Name SystemRestorePointCreationFrequency -ErrorAction SilentlyContinue).SystemRestorePointCreationFrequency
New-ItemProperty -Path $k -Name SystemRestorePointCreationFrequency -Value 0 -PropertyType DWord -Force | Out-Null
try { Enable-ComputerRestore -Drive ($env:SystemDrive + '\') -ErrorAction SilentlyContinue } catch {}
try { Checkpoint-Computer -Description ` + psQuote(label) + ` -RestorePointType MODIFY_SETTINGS -ErrorAction Stop; 'OK' }
finally { if($null -ne $old){ Set-ItemProperty -Path $k -Name SystemRestorePointCreationFrequency -Value $old } else { Remove-ItemProperty -Path $k -Name SystemRestorePointCreationFrequency -ErrorAction SilentlyContinue } }`
}

// parseJSONList reads a JSON array from PowerShell output, also accepting a lone
// object or string (older PowerShell sometimes unwraps one-element arrays).
func parseJSONList(out string, v interface{}) {
	out = strings.TrimSpace(out)
	i := strings.IndexAny(out, "[{\"")
	if i < 0 {
		return
	}
	body := out[i:]
	if body[0] != '[' {
		body = "[" + body + "]"
	}
	_ = json.Unmarshal([]byte(body), v)
}

// allScriptsForTest returns every script variant with sample arguments.
func allScriptsForTest() map[string]string {
	pairs := [][2]string{{"*EEE", "0"}, {"*InterruptModeration", "0"}}
	m := map[string]string{
		"tasks":        psApplyTasks([]string{`\Microsoft\Windows\Autochk\Proxy`, `NvTmRep*`, `\Microsoft\Windows\Application Experience\Microsoft Compatibility Appraiser`}),
		"tasksRevert":  psRevertTasks([]taskRec{{P: `\Microsoft\Windows\Autochk\`, N: "Proxy", S: "Ready"}, {P: `\`, N: "O'Brien task", S: "Ready"}}),
		"bitlocker":    psBitLocker(),
		"nic":          psNic(pairs, true),
		"nicNoPM":      psNic(pairs, false),
		"nicRevert":    psRevertNic(nicBackup{Props: []nicRec{{N: "Ethernet 2", K: "*EEE", V: "1"}}, PM: []string{"Wi-Fi"}}),
		"usb":          psUSBApply(),
		"usbRevert":    psUSBRevert([]string{`USB\ROOT_HUB30\4&2a8b1d5&0&0_0`, `HID\VID_046D&PID_C539\7&1'x_0`}),
		"mm":           psMMApply("MemoryCompression"),
		"mmRevert":     psMMRevert("PageCombining"),
		"dnsGet":       psDNSGet(),
		"dnsSet":       psDNSSet([]string{"1.1.1.1", "1.0.0.1"}),
		"dnsAuto":      psDNSSet(nil),
		"restorePoint": psRestorePoint("Astro Tweaks - before optimize"),
	}
	for _, t := range Catalog {
		for i, a := range t.Actions {
			if a.Kind == "ps" {
				m[fmt.Sprintf("catalog-%s-%d-apply", t.ID, i)] = a.Apply
				if a.Revert != "" {
					m[fmt.Sprintf("catalog-%s-%d-revert", t.ID, i)] = a.Revert
				}
			}
		}
	}
	for k, v := range m {
		m[k] = psPrefix + v
	}
	return m
}
