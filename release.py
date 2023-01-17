"""发布版本，参数分别是模块名称和版本号。执行以下操作：
    1. 使用sed将依赖该模块的其他模块依赖升级到当前版本号；
    2. 自动commit并且打上版本号的tag；
    3. 自动push上去；
    4. 自动到各个文件夹下运行`go get -u`下载最新的依赖；
    5. 各依赖模块也需要发布一个新版本，自动将小版本号加1并发布；
"""

import os
import platform
import sys

if len(sys.argv) < 3:
    print("usage: python release.py <module> <version>")
    exit()
module, version = sys.argv[1:3]
if not version.startswith('v'):
    version = "v" + version
print("prepare releasing %s %s" % (module, version))
cmd = r''' -i "s/{module}\ v.*/{module}\ {version}/g" `find . -name "go.mod"`'''
# mac上要用gnu版本，否则语法不兼容
if platform.system() == "Darwin":
    cmd = 'gsed' + cmd
else:
    cmd = 'sed' + cmd
cmd = cmd.format(module=module, version=version)
print(cmd)
if os.system(cmd):
    print("sed 失败")
    exit()
input("请检查确认修改是否无误，回车后自动提交")
os.system("git add . && git commit -m 'upgrade %s to %s'" % (module, version))
if os.system("git pull --rebase"):
    input("rebase失败，请手动处理之后再回车继续")
os.system("git tag %s/%s" % (module, version))
os.system("git push")
os.system("git push --tags")
for d in os.listdir("."):
    if d != module and os.path.isdir(d) and not d.startswith('.'):
        os.system('cd %s && go mod tidy && cd ..' % d)
        print('update %s dep...' % d)
os.system("git add . && git commit -m 'update modules dep of %s version to %s'" % (module, version))
os.system("git push")
# 更新依赖该模块的其他模块的版本
files = os.popen(r'grep "%s\ v" -rl --include=go.mod .' % module).read()
for f in files.split('\n'):
    if not f:
        continue
    mod = f.split('/')[1]
    tag = os.popen('git describe --match "%s/v*" --tags --abbrev=0' % mod).readlines()
    if not tag:
        print('not found mod %s tag, skip...' % mod)
    else:
        tag = str(tag[0]).strip()
        print('upgrade %s...' % tag)
        idx = tag.rfind('.') + 1
        prefix, minor = tag[:idx], tag[idx:]
        tag = prefix + str(int(minor) + 1)
        print('add tag %s' % tag)
        os.system('git tag %s' % tag)
os.system("git push --tags")
