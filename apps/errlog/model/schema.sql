create table if not exists error_reports (
	uuid text primary key not null,

	app         text     not null,
	machine     text     not null,


	happened_at datetime not null,
	status_code integer  not null,
	report      text     not null
);

create virtual table if not exists error_report_search using fts4(
	uuid   primary key references error_reports.uuid,

	app     text not null,
	machine text not null,

	report  text not null,

        notindexed=uuid
);

create index if not exists ams_time   on error_reports (app, machine, status_code, happened_at);
create index if not exists am_time    on error_reports (app, machine,              happened_at);
create index if not exists a_time     on error_reports (app,                       happened_at);

create index if not exists as_time    on error_reports (app,          status_code, happened_at);
create index if not exists a_time     on error_reports (app,                       happened_at);

create index if not exists ms_time    on error_reports (     machine, status_code, happened_at);
create index if not exists m_time     on error_reports (     machine,              happened_at);

create index if not exists time_time  on error_reports (                           happened_at);
