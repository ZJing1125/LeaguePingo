// Shared product copy. Keep new UI text here as existing sections are migrated.
export const uiText = {
  automationState: {
    loading: "正在读取设置…",
    unavailable: "设置暂未读取",
    retry: "重新读取设置",
  },
  connectionGuide: {
    title: "连接游戏客户端",
    steps: [
      "启动腾讯国服英雄联盟客户端",
      "完成登录，进入客户端首页",
      "保持 LeaguePingo 运行，等待自动连接",
    ],
  },
} as const;
