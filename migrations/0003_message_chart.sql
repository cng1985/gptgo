-- 问数结果里的图表数据。刷新对话后仍能画出同一张图。

ALTER TABLE message ADD COLUMN chart TEXT NOT NULL DEFAULT '';
