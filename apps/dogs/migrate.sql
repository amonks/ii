create table if not exists entries (
  number                integer primary key,
  date                  text,
  count                 real,
  eater                 text,
  photo_url             text,
  photo_filename        text,
  notes                 text,
  wordcount             integer
);

create index if not exists eater_date on entries (eater, date);
create index if not exists date on entries (date);
create index if not exists wordcount on entries (wordcount);

