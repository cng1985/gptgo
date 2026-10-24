-- 助手消息上的联网来源。检索摘要只在当轮交给模型，来源随消息保存。

ALTER TABLE message ADD COLUMN sources TEXT NOT NULL DEFAULT '';
