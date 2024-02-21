create table entries (
  number                integer primary key,
  date                  text,
  count                 real,
  eater                 text,
  photo_url             text,
  photo_filename        text,
  notes                 text,
  wordcount             integer
);

create index eater_date on entries (eater, date);
create index date on entries (date);
create index wordcount on entries (wordcount);

