select
  entries.eater as eater,
  sum(entries.count) as score
from
  entries
group by
  eater
order by
  score desc;
