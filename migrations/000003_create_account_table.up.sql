create table if not exists public.t_account
(
    id              bigserial               not null
        constraint t_account_pk
            primary key,
    difference      integer                 not null,
    user_id         bigint                  not null
        constraint t_account_t_user_id_fk
            references public.t_user,
    order_number    varchar,
    created_at      timestamp default now() not null,
    updated_at      timestamp default now() not null
);

comment on table public.t_account is 'Счёт бонусов пользователей';
comment on column public.t_account.id is 'Идентификатор транзакции';
comment on column public.t_account.difference is 'Изменение счёта в минимальных единицах (например, копейках или центах)';
comment on column public.t_account.user_id is 'Какому пользователю принадлежит';
comment on column public.t_account.order_number is 'Идентификатор связанного заказа (если есть)';

create index t_account_user_id_index on public.t_account (user_id);
create index t_account_order_number_index on public.t_account (order_number);