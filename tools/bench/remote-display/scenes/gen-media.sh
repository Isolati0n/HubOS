#!/bin/bash
# Makes the fixed input files for the scenes (same bytes every time): a text corpus and a short test video with a frame-number bar code.
. "$(dirname "$0")/../env.sh"
M=$BENCH_TMP/media; mkdir -p $M
if [ ! -s $M/corpus.txt ]; then
python3 - > $M/corpus.txt <<'PY'
import random
r = random.Random(42)
words = "the of and to in is that it for was on are as with his they at be this from have or by one had not but what all were when we there can an your which their said if do will each about how up out them then she many some so these would other into has more her two like him see time could no make than first been its who now people my made over did down only way find use may water long little very after words called just where most know".split()
extra = ["café", "naïve", "über", "señor", "Ωmega", "日本語", "żółć", "→", "✓", "€5"]
for i in range(6000):
    n = r.randint(6, 16)
    ws = [r.choice(words) for _ in range(n)]
    if i % 7 == 0: ws[r.randrange(n)] = r.choice(extra)
    print("%04d %s" % (i, " ".join(ws)))
PY
fi
if [ ! -s $M/video.mp4 ]; then
  # 1280x720, 30 fps, 40 s test pattern. The top 40 rows carry the frame number N as 16 black/white blocks of 80 px (bit k = block k).
  ffmpeg -v error -y -f lavfi -i "testsrc2=size=1280x720:rate=30:duration=40" -vf "format=yuv420p,geq=lum='if(lt(Y,40),if(gte(mod(floor(N/pow(2,floor(X/80))),2),1),235,16),lum(X,Y))':cb='if(lt(Y,20),128,cb(X,Y))':cr='if(lt(Y,20),128,cr(X,Y))'" -c:v libx264 -preset veryfast -crf 18 -g 30 $M/video.mp4
fi
ls -la $M
