create table public.t_user
(
    id            bigserial               not null
        constraint t_user_pk
            primary key,
    created_at    timestamp default now() not null,
    updated_at    timestamp default now() not null,
    login         varchar                 not null,
    password_hash varchar                 not null
);
comment on column public.t_user.id is 'Идентификатор пользователя';
comment on column public.t_user.created_at is 'Дата создания пользователя';
comment on column public.t_user.updated_at is 'Дата обновления пользователя';
comment on column public.t_user.login is 'Логин пользователя';
comment on column public.t_user.password_hash is 'Хэш пароля пользователя';
create unique index t_user_login_uindex on public.t_user (login);
