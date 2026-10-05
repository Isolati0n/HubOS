# Black box recorder LAB kernel: added AFTER image/kernel/qemu-test.frag (see build-lab.sh). Only what the experiment needs.
# The repo's own fragment has none of these (docs/proposals/black-box-recorder.md, section 2).
CONFIG_PSTORE=y
CONFIG_PSTORE_CONSOLE=y
CONFIG_PSTORE_PMSG=y
CONFIG_PSTORE_RAM=y
CONFIG_EFI_VARS_PSTORE=y
CONFIG_PRINTK_TIME=y
CONFIG_PROC_SYSCTL=y
