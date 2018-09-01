<?php


namespace Next\Database;


use Next\Database\Orm\Builder;

interface Scope
{
    public function apply(Model $model, Builder $builder): void ;
}