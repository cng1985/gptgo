-- 助手消息记下生成时的思考模式，刷新后仍按对应方式展示。

ALTER TABLE message ADD COLUMN thinking_mode TEXT NOT NULL DEFAULT '';
