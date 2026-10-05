. ${HS_WORK}/hs/env.sh
export S
H=$S/hubstab
rm -rf $H/smithay-p6 $H/dwp2
cp -r /root/.cargo/git/checkouts/smithay-312425d48e59d8c8/4cf0b62 $H/smithay-p6
rm -rf $H/smithay-p6/.git
# the one-line fix: return after posting the protocol error
python3 - <<'EOF'
import os
p = os.environ['S'] + '/hubstab/smithay-p6/src/wayland/shm/handlers.rs'
s = open(p).read()
old = '''                    pool.post_error(wl_shm::Error::InvalidFd, "invalid wl_shm_pool size");
                }
'''
new = '''                    pool.post_error(wl_shm::Error::InvalidFd, "invalid wl_shm_pool size");
                    return;
                }
'''
assert s.count(old) == 1
open(p, 'w').write(s.replace(old, new))
print('patched', p)
EOF
mkdir -p $H/dwp2 && cp -r $H/dwp/src $H/dwp/Cargo.toml $H/dwp/Cargo.lock $H/dwp/clippy.toml $H/dwp/assets $H/dwp/extras $H/dwp/config.reference.toml $H/dwp2/
cat >> $H/dwp2/Cargo.toml <<EOF

[patch."https://github.com/Smithay/smithay.git"]
smithay = { path = "$H/smithay-p6" }
smithay-drm-extras = { path = "$H/smithay-p6/smithay-drm-extras" }
EOF
cd $H/dwp2
export CARGO_TARGET_DIR=$H/target2
SECONDS=0
nice -n 10 cargo build --release -j2 2>&1 | tail -5
echo "P6 BUILD after ${SECONDS}s"
cp $CARGO_TARGET_DIR/release/driftwm $H/bin/driftwm-patched-p6
ls -la $H/bin/driftwm-patched-p6
