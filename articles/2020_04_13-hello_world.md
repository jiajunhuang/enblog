# Fix problems with Windows guest on virt manager(with KVM)

- My Windows client cann't change it's resolution automatically, what should I do?

Check if you have a Channel with it's type as `spicevmc` and value as `com.redhat.spice.0`, if it doesn't exist,
add it, shutdown your guest, and then restart it.

- My Windows client cann't change it's resolution automatically in a HiDPI screen, what can I do?

edit your Windows guest description XML file in virt manager or run it in command line:

```bash
$ sudo virsh
[sudo] password for jiajun:
Welcome to virsh, the virtualization interactive terminal.

Type:  'help' for help with commands
       'quit' to quit

virsh # edit --domain win
```

and find a section like this:

```
<video>
    <model type='qxl' ram='65536' vram='65536' vgamem='32768' heads='1' primary='yes'/>
    <address type='pci' domain='0x0000' bus='0x00' slot='0x01' function='0x0'/>
</video>
```

change value in vgamem='32768' to 32768 or 65536, then shutdown your guest, and restart it.
