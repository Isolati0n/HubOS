ulimit -c 0
export W=${BC_WORK:?set BC_WORK to your work folder}
export A=$W/A
export L=$A/root/usr/lib/x86_64-linux-gnu
export RUSTUP_HOME=$W/rustup
export CARGO_HOME=$W/cargo
export PATH=$W/cargo/bin:$PATH
export PKG_CONFIG_PATH=$L/pkgconfig:$A/root/usr/share/pkgconfig
export PKG_CONFIG_SYSROOT_DIR=
export LD_LIBRARY_PATH=$L
export LIBRARY_PATH=$L
export C_INCLUDE_PATH=$A/root/usr/include
export CPATH=$A/root/usr/include
export RUSTFLAGS_BASE="-L $L"
