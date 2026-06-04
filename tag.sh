#!/bin/sh

set -e

# 获取最新 tag
LATEST_TAG=$(git tag | sort -V | tail -n 1)

if [ -z "$LATEST_TAG" ]; then
    echo "未找到 Tag"
    exit 1
fi

echo "当前最新 Tag: $LATEST_TAG"

# 提取最后一段版本号
PREFIX=$(echo "$LATEST_TAG" | sed -E 's/\.[0-9]+$//')
LAST_NUM=$(echo "$LATEST_TAG" | sed -E 's/^.*\.([0-9]+)$/\1/')

NEW_NUM=$((LAST_NUM + 1))
NEW_TAG="${PREFIX}.${NEW_NUM}"

echo "新 Tag: $NEW_TAG"

# 创建 tag
git tag "$NEW_TAG"

# 推送 tag
git push origin "$NEW_TAG"

echo "Tag 发布成功: $NEW_TAG"